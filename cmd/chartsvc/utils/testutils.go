package utils

import (
	"bytes"
	"image/color"
	"local/monocular/cmd/chartsvc/models"

	"github.com/disintegration/imaging"
)

var ChartsList []*models.Chart

func IconBytes() []byte {
	var b bytes.Buffer
	img := imaging.New(1, 1, color.White)
	imaging.Encode(&b, img, imaging.PNG)
	return b.Bytes()
}

const TestChartReadme = "# Quickstart\n\n```bash\nhelm install my-repo/my-chart\n```"
const TestChartValues = "image:\n  registry: docker.io\n  repository: my-repo/my-chart\n  tag: 0.1.0"
