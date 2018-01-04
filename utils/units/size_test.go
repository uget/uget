package units

import "github.com/stretchr/testify/assert"
import "testing"

func Test(t *testing.T) {
	assert.Equal(t, "1000 KiB", BytesSize(1000*1024))
	assert.Equal(t, "999.0 KiB", BytesSize(999*1024))
	assert.Equal(t, "1.000 KiB", BytesSize(1*1024))
	assert.Equal(t, "10.00 KiB", BytesSize(10*1024))
	assert.Equal(t, "980.0 KiB", BytesSize(980*1024))
}
