package deposit

import (
	"github.com/amazechain/amc/common/crypto/bls"
	"github.com/amazechain/amc/common/hexutil"
	"testing"
)

func TestBLS(t *testing.T) {
	sig, _ := hexutil.Decode("0xad79339f8ae807f74f807d9800dcef83c3a149636b475c95d57e8e659743d0b01b239a7909839e0f1d078b374d80d6db13b1c9fee7e5a0ecc77e8fff219df68aaf77e3519897aecdc9e6796ab38adf7c03b1e8f440bff34ced8b201154749aa8")
	bp, _ := hexutil.Decode("0x804081df620122e924d969b9895221b0303913896316085fa8668cd462f66b1ede2542aefaf1459474994b4ffcb9ef23")
	msg, _ := hexutil.Decode("0x8ac7230489e80000") // 10 AMT

	signature, err := bls.SignatureFromBytes(sig)
	if err != nil {
		t.Fatal("cannot unpack BLS signature", err)
	}

	publicKey, err := bls.PublicKeyFromBytes(bp)

	if err != nil {
		t.Fatal("cannot unpack BLS publicKey", err)
	}

	if !signature.Verify(publicKey, msg) {
		t.Fatal("bls cannot verify signature")
	}

	private, _ := hexutil.Decode("0xde4b76c3dca3d8e10aea7644f77b316a68a6476fbd119d441ead5c6131aa42a7")

	p, err := bls.SecretKeyFromBytes(private)
	if err != nil {
		t.Fatal("bls cannot import private key", err)
	}

	pub, err := p.PublicKey().MarshalText()

	if err != nil {
		t.Fatal("bls public key cannot MarshalText ", err)
	}
	t.Logf("pubkey %s", string(pub))

}
