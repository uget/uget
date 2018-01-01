package units

import "github.com/stretchr/testify/assert"
import "testing"

func Test(t *testing.T) {
	assert.Equal(t, BytesSize(1000*1024), "1000 KiB")
	assert.Equal(t, BytesSize(999*1024), "999.0 KiB")
	assert.Equal(t, BytesSize(1*1024), "1.000 KiB")
	assert.Equal(t, BytesSize(10*1024), "10.00 KiB")
	assert.Equal(t, BytesSize(980*1024), "980.0 KiB")
}
