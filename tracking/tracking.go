package tracking

import (
	mixpanel "github.com/ajones/go-mixpanel"
	"github.com/inburst/prty/logger"
)

const PUBLIC_MIXPANEL_TOKEN = "cfbcac175794b64492fee5160288c406"

func SendMetric(metricName string) {
	mp := mixpanel.NewMixpanel(PUBLIC_MIXPANEL_TOKEN)
	err := mp.Track(mixpanel.NewEvent(metricName))
	if err != nil {
		logger.Shared().Printf("%s", err)
	}
}
