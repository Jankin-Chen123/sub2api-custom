import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import channelMonitorV2 from './channelMonitorV2'
import batchImage from './batchImage'
import imageWorkbench from './imageWorkbench'
import admin from './admin'
import misc from './misc'
import contactPage from './contactPage'

export default {
  ...landing,
  ...common,
  ...dashboard,
  ...channelMonitorV2,
  ...batchImage,
  ...imageWorkbench,
  admin,
  ...contactPage,
  ...misc,
}
