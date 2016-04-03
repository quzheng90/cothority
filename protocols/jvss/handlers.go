package jvss

import (
	"fmt"

	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/poly"
)

// SecInitMsg are used to initialise new shared secrets both long- and
// short-term.
type SecInitMsg struct {
	Src  int
	SID  string
	Deal []byte
}

// SecConfMsg are used to confirm to other peers that we have finished setting
// up the shared secret.
type SecConfMsg struct {
	Src int
	SID string
}

// SigReqMsg are used to send signing requests.
type SigReqMsg struct {
	Src int
	SID string
	Msg []byte
}

// SigRespMsg are used to reply to signing requests.
type SigRespMsg struct {
	Src int
	SID string
	Sig *poly.SchnorrPartialSig
}

// WSecInitMsg is a SDA-wrapper around SecInitMsg.
type WSecInitMsg struct {
	*sda.TreeNode
	SecInitMsg
}

// WSecConfMsg is a SDA-wrapper around SecConfMsg.
type WSecConfMsg struct {
	*sda.TreeNode
	SecConfMsg
}

// WSigReqMsg is a SDA-wrapper around SigReqMsg.
type WSigReqMsg struct {
	*sda.TreeNode
	SigReqMsg
}

// WSigRespMsg is a SDA-wrapper around SigRespMsg.
type WSigRespMsg struct {
	*sda.TreeNode
	SigRespMsg
}

func (jv *JVSS) handleSecInit(m WSecInitMsg) error {
	msg := m.SecInitMsg

	// Initialise shared secret
	if err := jv.initSecret(msg.SID); err != nil {
		return err
	}

	// Unmarshal received deal
	deal := new(poly.Deal).UnmarshalInit(jv.info.T, jv.info.R, jv.info.N, jv.keyPair.Suite)
	if err := deal.UnmarshalBinary(msg.Deal); err != nil {
		return err
	}

	// Buffer received deal for later
	secret, ok := jv.secrets[msg.SID]
	if !ok {
		return fmt.Errorf("Error, shared secret does not exist")
	}
	secret.deals[msg.Src] = deal

	//dbg.Lvl1(fmt.Sprintf("Node %d: received %s deal %d/%d from %d", jv.nodeIdx(), msg.SID, len(secret.deals), len(jv.nodeList), msg.Src))

	// Finalise shared secret
	if err := jv.finaliseSecret(msg.SID); err != nil {
		return err
	}

	return nil
}

func (jv *JVSS) handleSecConf(m WSecConfMsg) error {
	msg := m.SecConfMsg

	secret, ok := jv.secrets[msg.SID]
	if !ok {
		return fmt.Errorf("Error, shared secret does not exist")
	}

	secret.mtx.Lock()
	secret.numConfs++
	secret.mtx.Unlock()

	//dbg.Lvl1(fmt.Sprintf("Node %d: %s confirmations %d/%d", jv.nodeIdx(), msg.SID, secret.numConfs, len(jv.nodeList)))

	// Check if we have enough confirmations to proceed
	if (secret.numConfs == len(jv.nodeList)) && (msg.SID == LTSS || msg.SID == fmt.Sprintf("%s%d", STSS, jv.nodeIdx())) {
		jv.secretsDone <- true
	}

	return nil
}

func (jv *JVSS) handleSigReq(m WSigReqMsg) error {
	msg := m.SigReqMsg

	// Create partial signature
	ps, err := jv.sigPartial(msg.SID, msg.Msg)
	if err != nil {
		return err
	}

	// Send it back to initiator
	resp := &SigRespMsg{
		Src: jv.nodeIdx(),
		SID: msg.SID,
		Sig: ps,
	}

	if err := jv.SendTo(jv.nodeList[msg.Src], resp); err != nil {
		return err
	}

	// Cleanup short-term shared secret
	delete(jv.secrets, msg.SID)

	return nil
}

func (jv *JVSS) handleSigResp(m WSigRespMsg) error {
	msg := m.SigRespMsg

	// Collect partial signatures
	secret, ok := jv.secrets[msg.SID]
	if !ok {
		return fmt.Errorf("Error, shared secret does not exist")
	}
	secret.sigs[msg.Src] = msg.Sig

	//dbg.Lvl1(fmt.Sprintf("Node %d: %s signatures %d/%d (from %d)", jv.nodeIdx(), msg.SID, secret.numSigs, len(jv.nodeList), msg.Src))

	// Create Schnorr signature once we received enough replies
	if jv.info.T == len(secret.sigs) {

		for i := 0; i < len(secret.sigs); i++ {
			if err := jv.schnorr.AddPartialSig(secret.sigs[i]); err != nil {
				return err
			}
		}

		sig, err := jv.schnorr.Sig()
		if err != nil {
			return err
		}
		jv.sigChan <- sig

		// Cleanup short-term shared secret
		delete(jv.secrets, msg.SID)
	}

	return nil
}
