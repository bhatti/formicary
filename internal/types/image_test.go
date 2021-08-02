package types

import (
	"github.com/stretchr/testify/require"
	"testing"
)


func Test_ShouldGetImagePorts(t *testing.T) {
	image := Image{
		Ports: []Port {
			{
				Number: 1,
				Protocol: "",
			},
			{
				Number: 2,
				Protocol: "tcp",
			},
		},
	}
	_, ports := image.GetPorts()
	require.Equal(t, 2, len(ports))
}

