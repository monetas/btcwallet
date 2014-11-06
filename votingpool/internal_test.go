package votingpool

import (
	"github.com/conformal/btcutil"
	"github.com/conformal/btcwallet/txstore"
	"github.com/conformal/btcwallet/waddrmgr"
)

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
func TstRunWithReplacedCryptoKeyScript(p *VotingPool,
	encryptScript func([]byte) ([]byte, error), callback func()) {
	orig := p.encryptScript
	defer func() { p.encryptScript = orig }()
	p.encryptScript = encryptScript
	callback()
}

// TstDecryptExtendedKey exposes the private decryptExtendedKey for
// testing.
var TstDecryptExtendedKey = decryptExtendedKey

// TstEligible exposes the private votingpool method eligible for
// testing.
func (vp *VotingPool) TstEligible(c txstore.Credit, minConf int, chainHeight int32, dustThreshold btcutil.Amount) bool {
	return vp.eligible(c, minConf, chainHeight, dustThreshold)
}

// TstGetEligibleInputsFromSeries exposes the private votingpool
// method getEligibleInputsFromSeries for testing.
func (vp *VotingPool) TstGetEligibleInputsFromSeries(store *txstore.Store,
	sRange AddressRange,
	dustThreshold btcutil.Amount, chainHeight int32,
	minConf int) (Credits, error) {
	return vp.getEligibleInputsFromSeries(store, sRange, dustThreshold, chainHeight, minConf)
}

// TstGetEligibleInputs exposes the private votingpool method
// getEligibleInputs for testing.
func (vp *VotingPool) TstGetEligibleInputs(store *txstore.Store,
	sRanges []AddressRange,
	dustThreshold btcutil.Amount, chainHeight int32,
	minConf int) (Credits, error) {
	return vp.getEligibleInputs(store, sRanges, dustThreshold, chainHeight, minConf)
}
