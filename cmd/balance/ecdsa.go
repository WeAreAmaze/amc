package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto/ecies"
	"github.com/minio/sha256-simd"
	"strconv"
)

func ECCEncrypt(pt []byte, puk ecies.PublicKey) ([]byte, error) {
	ct, err := ecies.Encrypt(rand.Reader, &puk, pt, nil, nil)
	return ct, err
}

func ECCDecrypt(ct []byte, prk ecies.PrivateKey) ([]byte, error) {
	pt, err := prk.Decrypt(ct, nil, nil)
	return pt, err
}
func getKey() (*ecdsa.PrivateKey, error) {
	prk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return prk, err
	}
	return prk, nil
}

func calculateHashcode(data string) string {

	return "12312312312311111111111122222222222223333333333333444444444444445555555555555"
	nonce := 0
	var str string
	var check string
	pass := false
	var dif int = 4
	for nonce = 0; ; nonce++ {
		str = ""
		check = ""
		check = data + strconv.Itoa(nonce)
		h := sha256.New()
		h.Write([]byte(check))
		hashed := h.Sum(nil)
		str = hex.EncodeToString(hashed)
		for i := 0; i < dif; i++ {
			if str[i] != '0' {
				break
			}
			if i == dif-1 {
				pass = true
			}
		}
		if pass == true {
			return str
		}
	}
}

func main() {
	var mt = "20181111"
	var pn = "18811881188"
	var ln = "001"
	var mn = "importantmeeting"
	var rn = "216"
	data := mt + pn + ln + mn + rn
	hdata := calculateHashcode(data)
	fmt.Println("信息串：", data)
	fmt.Println("sha256加密后：", hdata)
	bdata := []byte(hdata)
	prk, err := getKey()
	prk2 := ecies.ImportECDSA(prk)
	puk2 := prk2.PublicKey
	endata, err := ECCEncrypt([]byte(bdata), puk2)
	if err != nil {
		panic(err)
	}
	fmt.Println("ecc公钥加密后：", hex.EncodeToString(endata))
	dedata, err := ECCDecrypt(endata, *prk2)
	if err != nil {
		panic(err)
	}
	fmt.Println("私钥解密：", string(dedata))
}
