package crypto

import "testing"

func TestKeccak256Hash(t *testing.T) {
	t.Log(Keccak256Hash(nil))
}
