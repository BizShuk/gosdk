package metric

import (
    "testing"
)

func TestMimirService_SendTest(t *testing.T) {
    svc := NewMimirService()
    if err := svc.SendTest(); err != nil {
        t.Errorf("SendTest() error = %v", err)
    }
}