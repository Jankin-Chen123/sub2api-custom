package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ImageGenerationJob is the durable task source of truth for a single image
// generation or edit. Raw prompts and input image bytes deliberately live in a
// separate encrypted, TTL-controlled payload store and are referenced only by
// payload_object_ref.
type ImageGenerationJob struct {
	ent.Schema
}

func (ImageGenerationJob) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "image_generation_jobs"},
	}
}

func (ImageGenerationJob) Fields() []ent.Field {
	return []ent.Field{
		field.String("job_id").MaxLen(64).Immutable(),
		field.Int64("user_id").Optional().Nillable(),
		field.Int64("api_key_id").Optional().Nillable(),
		field.Int64("group_id").Optional().Nillable(),
		field.Int64("subscription_id").Optional().Nillable(),
		field.Int64("account_id").Optional().Nillable(),
		field.Int8("billing_type").Default(0),
		field.String("source").MaxLen(32),
		field.String("operation").MaxLen(32),
		field.String("status").MaxLen(32).Default("created"),
		field.String("public_model").MaxLen(128),
		field.String("upstream_model").Optional().Nillable().MaxLen(128),
		field.String("requested_size").Optional().Nillable().MaxLen(32),
		field.String("actual_size").Optional().Nillable().MaxLen(32),
		field.String("quality").Optional().Nillable().MaxLen(32),
		field.String("response_format").Optional().Nillable().MaxLen(32),
		field.String("upstream_task_id").Optional().Nillable().MaxLen(512),
		field.String("idempotency_key").Optional().Nillable().MaxLen(255),
		field.String("request_hash").Optional().Nillable().MaxLen(128),
		field.String("prompt_hash").MaxLen(128),
		field.String("payload_object_ref").Optional().Nillable().MaxLen(1024),
		field.JSON("result_object_refs", []string{}).Default([]string{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Float("base_cost").SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).Default(0),
		field.Float("rate_multiplier").SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).Default(1),
		field.Float("estimated_cost").SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).Default(0),
		field.Float("held_cost").SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).Default(0),
		field.Float("settled_cost").SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).Default(0),
		field.String("error_code").Optional().Nillable().MaxLen(128),
		field.String("error_message").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int("attempt_count").Default(0),
		field.Int64("claim_version").Default(0),
		field.Time("lease_expires_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("next_attempt_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("submitted_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("completed_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("settled_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (ImageGenerationJob) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("job_id").Unique(),
		index.Fields("user_id", "created_at"),
		index.Fields("api_key_id", "created_at"),
		index.Fields("account_id", "status"),
		index.Fields("status", "next_attempt_at", "created_at"),
		index.Fields("lease_expires_at"),
		index.Fields("account_id", "upstream_task_id"),
		index.Fields("completed_at"),
	}
}
