package twikey

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func generateOtp(_salt string, _privKey string) (int, error) {

	salt := []byte(_salt)
	privkey, err := hex.DecodeString(_privKey)

	if err != nil {
		return 0, err
	}

	total := len(salt) + len(privkey)
	key := make([]byte, total, total)
	copy(key, salt)
	copy(key[len(salt):], privkey)

	buf := make([]byte, 8)
	_time := time.Now().UTC().Unix() //*1000
	counter := uint64(math.Floor(float64(_time / 30)))
	binary.BigEndian.PutUint64(buf, counter)

	mac := hmac.New(sha256.New, key)
	mac.Write(buf)
	hash := mac.Sum(nil)

	offset := hash[19] & 0xf
	v := int64(((int(hash[offset]) & 0x7f) << 24) |
		((int(hash[offset+1] & 0xff)) << 16) |
		((int(hash[offset+2] & 0xff)) << 8) |
		(int(hash[offset+3]) & 0xff))

	// last 8 digits are important
	return int(v % 100000000), nil
}

func (c *TwikeyClient) refreshTokenIfRequired() error {

	if time.Now().Sub(c.lastLogin).Hours() < 23 {
		return nil
	}

	params := url.Values{}
	params.Add("apiToken", c.ApiKey)
	if c.PrivateKey != "" {
		otp, _ := generateOtp(c.Salt, c.PrivateKey)
		params.Add("otp", fmt.Sprint(otp))
	}

	c.debug("Connecting to", c.BaseURL, " with ", c.ApiKey)

	req, err := http.NewRequest("POST", c.BaseURL+"/creditor", strings.NewReader(params.Encode()))
	if err == nil {
		req.Header.Set("User-Agent", c.UserAgent)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := c.HTTPClient.Do(req)
		if err == nil {
			token := resp.Header["Authorization"]
			if resp.StatusCode == 200 && token != nil {
				c.debug("Connected to", c.BaseURL, "with token", token[0])
				c.apiToken = token[0]
				c.lastLogin = time.Now()
				return nil
			} else if resp.StatusCode > 200 {
				err = errors.New(resp.Status)
			} else if resp.Header["Apierrorcode"][0] == "403" {
				errcode := resp.Header["Apierrorcode"][0]
				c.error("Invalid apiToken status =", errcode)
				err = errors.New("Invalid apiToken")
			}
		}
		c.apiToken = ""
		c.lastLogin = time.Unix(0, 0)
		return err
	} else {
		c.error("Not connected :", err)
		return err
	}
}

func (c *TwikeyClient) logout() {
	req, _ := http.NewRequest("GET", c.BaseURL+"/creditor", nil)
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", c.apiToken)

	res, _ := c.HTTPClient.Do(req)
	if res.StatusCode != 200 {
		c.error("Invalid logout from Twikey:", res.StatusCode)
	}
}
