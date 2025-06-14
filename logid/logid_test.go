package logid

import (
	. "github.com/bytedance/mockey"
	"testing"
	osTime "time"
)

func TestLogID(t *testing.T) {
	PatchConvey("test LogID", t, func() {
		{
			t.Logf("GetID with time.Now")
			for i := 0; i < 100; i++ {
				t.Logf("logID:%d", GetID())
			}
		}
		{
			t.Logf("GetID with osTime.Now")
			ld := NewLogID(osTime.Now)
			for i := 0; i < 100; i++ {
				t.Logf("logID:%d", ld.GetID())
			}
		}

		{
			t.Logf("GetID diff time")
			id := GetID()
			t.Logf("logID:%d", id)
			t.Logf("logID:0x%x", id)
			osTime.Sleep(osTime.Second)
			newID := GetID()
			t.Logf("logID:%d", newID)
			t.Logf("logID:0x%x", newID)
		}
	})
}
