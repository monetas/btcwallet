package votingpool

import "github.com/conformal/btcwallet/waddrmgr"

// TstPutSeries transparently wraps the voting pool putSeries method.
func (vp *VotingPool) TstPutSeries(version, seriesID, reqSigs uint32, inRawPubKeys []string) error {
	return vp.putSeries(version, seriesID, reqSigs, inRawPubKeys)
}

var TstBranchOrder = branchOrder

// TstExistsSeries checks whether a series is stored in the database.
// Used by the series creation test.
func (vp *VotingPool) TstExistsSeries(seriesID uint32) (bool, error) {
	return waddrmgr.ExistsSeries(vp.manager, vp.ID, seriesID)
}

// EncryptWithCryptoKeyPub allows using the manager's crypto key used for
// encryption of public keys.
func EncryptWithCryptoKeyPub(m *waddrmgr.Manager) func([]byte) ([]byte, error) {
	fun := func(unencrypted []byte) ([]byte, error) {
		return m.CryptoKeyPub().Encrypt([]byte(unencrypted))
	}
	return fun
}

// EncryptWithCryptoKeyPriv allows using the manager's crypto key used for
// encryption of private keys.
func EncryptWithCryptoKeyPriv(m *waddrmgr.Manager) func([]byte) ([]byte, error) {
	fun := func(unencrypted []byte) ([]byte, error) {
		return m.CryptoKeyPriv().Encrypt([]byte(unencrypted))
	}
	return fun
}

// TstGetRawPublicKeys gets a series public keys in string format.
func (s *seriesData) TstGetRawPublicKeys() []string {
	rawKeys := make([]string, len(s.publicKeys))
	for i, key := range s.publicKeys {
		rawKeys[i] = key.String()
	}
	return rawKeys
}

// TstGetRawPrivateKeys gets a series private keys in string format.
func (s *seriesData) TstGetRawPrivateKeys() []string {
	rawKeys := make([]string, len(s.privateKeys))
	for i, key := range s.privateKeys {
		if key != nil {
			rawKeys[i] = key.String()
		}
	}
	return rawKeys
}

// TstGetReqSigs expose the series reqSigs attribute.
func (s *seriesData) TstGetReqSigs() uint32 {
	return s.reqSigs
}

// TstEmptySeriesLookup empties the voting pool seriesLookup attribute.
func (vp *VotingPool) TstEmptySeriesLookup() {
	vp.seriesLookup = make(map[uint32]*seriesData)
}

var TstValidateAndDecryptKeys = validateAndDecryptKeys

// Replace Manager.cryptoKeyScript with the given one and calls the given function,
// resetting Manager.cryptoKeyScript to its original value after that.
func TstRunWithReplacedCryptoKeyScript(p *VotingPool, cryptoKey waddrmgr.EncryptorDecryptor, callback func()) {
	orig := p.cryptoKeyScript
	defer func() { p.cryptoKeyScript = orig }()
	p.cryptoKeyScript = func() waddrmgr.EncryptorDecryptor {
		return cryptoKey
	}
	callback()
}

var TstDecryptExtendedKey = decryptExtendedKey
