//go:build unit

package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestBuildPaymentOrderProviderSnapshot_ExcludesSensitiveConfig(t *testing.T) {
	t.Parallel()

	sel := &payment.InstanceSelection{
		InstanceID:     "12",
		ProviderKey:    payment.TypeWxpay,
		SupportedTypes: "wxpay,wxpay_direct",
		PaymentMode:    "popup",
		Config: map[string]string{
			"privateKey": "secret",
			"apiV3Key":   "secret-v3",
			"appId":      "wx-app-id",
		},
	}

	snapshot := buildPaymentOrderProviderSnapshot(sel, CreateOrderRequest{})
	require.Equal(t, map[string]any{
		"schema_version":       2,
		"provider_instance_id": "12",
		"provider_key":         payment.TypeWxpay,
		"payment_mode":         "popup",
		"merchant_app_id":      "wx-app-id",
		"currency":             "CNY",
	}, snapshot)
	require.NotContains(t, snapshot, "config")
	require.NotContains(t, snapshot, "privateKey")
	require.NotContains(t, snapshot, "apiV3Key")
	require.NotContains(t, snapshot, "supported_types")
	require.NotContains(t, snapshot, "instance_name")
	require.NotContains(t, snapshot, "merchant_id")
}

func TestCreateOrderInTx_WritesProviderSnapshot(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	campaignStart, _ := NewcomerCampaignWindow()

	user, err := client.User.Create().
		SetEmail("snapshot@example.com").
		SetPasswordHash("hash").
		SetUsername("snapshot-user").
		SetCreatedAt(campaignStart.Add(time.Hour)).
		Save(ctx)
	require.NoError(t, err)

	instance, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("Primary Alipay").
		SetConfig(`{"secretKey":"do-not-copy"}`).
		SetSupportedTypes("alipay,alipay_direct").
		SetPaymentMode("redirect").
		SetEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{entClient: client, now: func() time.Time { return campaignStart.Add(time.Hour) }}
	order, err := svc.createOrderInTx(
		ctx,
		CreateOrderRequest{
			UserID:      user.ID,
			Amount:      88,
			PaymentType: payment.TypeAlipay,
			OrderType:   payment.OrderTypeBalance,
			ClientIP:    "127.0.0.1",
			SrcHost:     "app.example.com",
		},
		&User{
			ID:       user.ID,
			Email:    user.Email,
			Username: user.Username,
		},
		nil,
		&PaymentConfig{
			MaxPendingOrders: 3,
			OrderTimeoutMin:  30,
		},
		105.6,
		88,
		0,
		90.2,
		"CNY",
		&payment.InstanceSelection{
			InstanceID:     strconv.FormatInt(instance.ID, 10),
			ProviderKey:    payment.TypeAlipay,
			SupportedTypes: "alipay,alipay_direct",
			PaymentMode:    "redirect",
			Config: map[string]string{
				"secretKey": "do-not-copy",
			},
		},
	)
	require.NoError(t, err)
	require.Equal(t, strconv.FormatInt(instance.ID, 10), valueOrEmpty(order.ProviderInstanceID))
	require.Equal(t, payment.TypeAlipay, valueOrEmpty(order.ProviderKey))
	require.Equal(t, float64(2), order.ProviderSnapshot["schema_version"])
	require.Equal(t, strconv.FormatInt(instance.ID, 10), order.ProviderSnapshot["provider_instance_id"])
	require.Equal(t, payment.TypeAlipay, order.ProviderSnapshot["provider_key"])
	require.Equal(t, "redirect", order.ProviderSnapshot["payment_mode"])
	require.NotContains(t, order.ProviderSnapshot, "config")
	require.NotContains(t, order.ProviderSnapshot, "secretKey")
	require.NotContains(t, order.ProviderSnapshot, "supported_types")
	require.NotContains(t, order.ProviderSnapshot, "instance_name")
	rows, err := client.QueryContext(ctx, "SELECT principal_amount, principal_currency FROM newcomer_campaign_payment_facts WHERE order_id = ?", order.ID)
	require.NoError(t, err)
	defer rows.Close()
	require.True(t, rows.Next())
	var principalAmount float64
	var principalCurrency string
	require.NoError(t, rows.Scan(&principalAmount, &principalCurrency))
	require.Equal(t, 88.0, principalAmount, "campaign fact must use req.Amount, not credited amount or pay_amount")
	require.Equal(t, "CNY", principalCurrency)
}

func TestCreateOrderInTx_PaymentFactHonorsCampaignWindowAndCaptureCutoff(t *testing.T) {
	ctx := context.Background()
	start, end := NewcomerCampaignWindow()
	captureEnd := end.Add(newcomerCampaignCaptureGrace)
	client := newPaymentConfigServiceTestClient(t)

	create := func(t *testing.T, email string, userCreatedAt, now time.Time) int64 {
		t.Helper()
		user, err := client.User.Create().
			SetEmail(email).
			SetPasswordHash("hash").
			SetUsername(email).
			SetCreatedAt(userCreatedAt).
			Save(ctx)
		require.NoError(t, err)
		svc := &PaymentService{entClient: client, now: func() time.Time { return now }}
		order, err := svc.createOrderInTx(ctx, CreateOrderRequest{
			UserID: user.ID, Amount: 10, PaymentType: payment.TypeAlipay,
			OrderType: payment.OrderTypeBalance, ClientIP: "127.0.0.1", SrcHost: "test",
		}, &User{ID: user.ID, Email: email, Username: email}, nil,
			&PaymentConfig{MaxPendingOrders: 3, OrderTimeoutMin: 30},
			10, 10, 0, 10, "CNY", nil)
		require.NoError(t, err)
		return order.ID
	}

	beforeID := create(t, "campaign-fact-before@example.invalid", start.Add(-time.Hour), start.Add(-time.Minute))
	insideID := create(t, "campaign-fact-inside@example.invalid", start.Add(time.Hour), start.Add(2*time.Hour))
	afterID := create(t, "campaign-fact-after@example.invalid", start.Add(time.Hour), captureEnd.Add(time.Hour))

	countFacts := func(orderID int64) int {
		rows, err := client.QueryContext(ctx, "SELECT COUNT(*) FROM newcomer_campaign_payment_facts WHERE order_id = ?", orderID)
		require.NoError(t, err)
		defer func() { _ = rows.Close() }()
		require.True(t, rows.Next())
		var count int
		require.NoError(t, rows.Scan(&count))
		require.NoError(t, rows.Err())
		return count
	}
	beforeCount := countFacts(beforeID)
	insideCount := countFacts(insideID)
	afterCount := countFacts(afterID)
	require.Equal(t, 0, beforeCount, "orders before campaign start must not create campaign facts")
	require.Equal(t, 1, insideCount, "orders for campaign users inside the window must create campaign facts")
	require.Equal(t, 0, afterCount, "orders after campaign capture cutoff must not create campaign facts")
}

func TestBuildPaymentOrderProviderSnapshot_UsesWxpayJSAPIAppIDForOpenIDOrders(t *testing.T) {
	t.Parallel()

	snapshot := buildPaymentOrderProviderSnapshot(&payment.InstanceSelection{
		InstanceID:  "88",
		ProviderKey: payment.TypeWxpay,
		Config: map[string]string{
			"appId":   "wx-open-app",
			"mpAppId": "wx-mp-app",
			"mchId":   "mch-88",
		},
		PaymentMode: "jsapi",
	}, CreateOrderRequest{OpenID: "openid-123"})

	require.Equal(t, "wx-mp-app", snapshot["merchant_app_id"])
	require.Equal(t, "mch-88", snapshot["merchant_id"])
	require.Equal(t, "CNY", snapshot["currency"])
}

func TestBuildPaymentOrderProviderSnapshot_IncludesAlipayMerchantIdentity(t *testing.T) {
	t.Parallel()

	snapshot := buildPaymentOrderProviderSnapshot(&payment.InstanceSelection{
		InstanceID:  "21",
		ProviderKey: payment.TypeAlipay,
		Config: map[string]string{
			"appId":      "alipay-app-21",
			"privateKey": "secret",
		},
		PaymentMode: "redirect",
	}, CreateOrderRequest{})

	require.Equal(t, "alipay-app-21", snapshot["merchant_app_id"])
	require.NotContains(t, snapshot, "privateKey")
}

func TestBuildPaymentOrderProviderSnapshot_IncludesEasyPayMerchantIdentity(t *testing.T) {
	t.Parallel()

	snapshot := buildPaymentOrderProviderSnapshot(&payment.InstanceSelection{
		InstanceID:  "66",
		ProviderKey: payment.TypeEasyPay,
		Config: map[string]string{
			"pid":  "easypay-merchant-66",
			"pkey": "secret",
		},
		PaymentMode: "popup",
	}, CreateOrderRequest{PaymentType: payment.TypeAlipay})

	require.Equal(t, "easypay-merchant-66", snapshot["merchant_id"])
	require.NotContains(t, snapshot, "pkey")
}

func TestBuildPaymentOrderProviderSnapshot_IncludesProviderCurrency(t *testing.T) {
	t.Parallel()

	stripeSnapshot := buildPaymentOrderProviderSnapshot(&payment.InstanceSelection{
		InstanceID:  "77",
		ProviderKey: payment.TypeStripe,
		Config: map[string]string{
			"currency": "hkd",
		},
	}, CreateOrderRequest{})
	require.Equal(t, "HKD", stripeSnapshot["currency"])

	airwallexSnapshot := buildPaymentOrderProviderSnapshot(&payment.InstanceSelection{
		InstanceID:  "78",
		ProviderKey: payment.TypeAirwallex,
		Config: map[string]string{
			"currency":  "usd",
			"accountId": "acct-78",
		},
	}, CreateOrderRequest{})
	require.Equal(t, "USD", airwallexSnapshot["currency"])
	require.Equal(t, "acct-78", airwallexSnapshot["merchant_id"])
}

func valueOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
