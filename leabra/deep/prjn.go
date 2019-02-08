// Copyright (c) 2019, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package deep

import (
	"github.com/emer/emergent/emer"
	"github.com/emer/emergent/leabra/leabra"
	"github.com/goki/ki/kit"
)

// deep.Prjn is the DeepLeabra projection, based on basic rate-coded leabra.Prjn
type Prjn struct {
	leabra.Prjn
	AttnGeInc     []float32 `desc:"local increment accumulator for AttnGe excitatory conductance from sending units -- this will be thread-safe"`
	TRCBurstGeInc []float32 `desc:"local increment accumulator for TRCBurstGe excitatory conductance from sending units -- this will be thread-safe"`
}

// AsLeabra returns this prjn as a leabra.Prjn -- all derived prjns must redefine
// this to return the base Prjn type, so that the LeabraPrjn interface does not
// need to include accessors to all the basic stuff.
func (pj *Prjn) AsLeabra() *leabra.Prjn {
	return &pj.Prjn
}

func (pj *Prjn) Defaults() {
	pj.Prjn.Defaults()
}

func (pj *Prjn) UpdateParams() {
	pj.Prjn.UpdateParams()
}

func (pj *Prjn) Build() error {
	err := pj.Prjn.Build()
	if err != nil {
		return err
	}
	rsh := pj.Recv.LayShape()
	rlen := rsh.Len()
	pj.AttnGeInc = make([]float32, rlen)
	pj.TRCBurstGeInc = make([]float32, rlen)
	return nil
}

//////////////////////////////////////////////////////////////////////////////////////
//  Init methods

func (pj *Prjn) InitWts() {
	pj.Prjn.InitWts()
	pj.InitGeInc()
}

func (pj *Prjn) InitGeInc() {
	pj.Prjn.InitGeInc()
	for ri := range pj.AttnGeInc {
		pj.AttnGeInc[ri] = 0
		pj.TRCBurstGeInc[ri] = 0
	}
}

//////////////////////////////////////////////////////////////////////////////////////
//  Act methods

// SendAttnGeDelta sends the delta-activation from sending neuron index si,
// to integrate into AttnGeInc excitatory conductance on receivers
func (pj *Prjn) SendAttnGeDelta(si int, delta float32) {
	scdel := delta * pj.GeScale
	nc := pj.SConN[si]
	st := pj.SConIdxSt[si]
	syns := pj.Syns[st : st+nc]
	scons := pj.SConIdx[st : st+nc]
	for ci := range syns {
		ri := scons[ci]
		pj.AttnGeInc[ri] += scdel * syns[ci].Wt
	}
}

// SendTRCBurstGeDelta sends the delta-DeepBurst activation from sending neuron index si,
// to integrate TRCBurstGe excitatory conductance on receivers
func (pj *Prjn) SendTRCBurstGeDelta(si int, delta float32) {
	scdel := delta * pj.GeScale
	nc := pj.SConN[si]
	st := pj.SConIdxSt[si]
	syns := pj.Syns[st : st+nc]
	scons := pj.SConIdx[st : st+nc]
	for ci := range syns {
		ri := scons[ci]
		pj.TRCBurstGeInc[ri] += scdel * syns[ci].Wt
	}
}

// RecvAttnGeInc increments the receiver's AttnGe from that of all the projections
func (pj *Prjn) RecvAttnGeInc() {
	rlay := pj.Recv.(*Layer)
	for ri := range rlay.DeepNeurs {
		rn := &rlay.DeepNeurs[ri]
		rn.AttnGe += pj.AttnGeInc[ri]
		pj.AttnGeInc[ri] = 0
	}
}

// RecvTRCBurstGeInc increments the receiver's TRCBurstGe from that of all the projections
func (pj *Prjn) RecvTRCBurstGeInc() {
	rlay := pj.Recv.(*Layer)
	for ri := range rlay.DeepNeurs {
		rn := &rlay.DeepNeurs[ri]
		rn.TRCBurstGe += pj.TRCBurstGeInc[ri]
		pj.TRCBurstGeInc[ri] = 0
	}
}

//////////////////////////////////////////////////////////////////////////////////////
//  Learn methods

//////////////////////////////////////////////////////////////////////////////////////
//  PrjnType

// DeepLeabra extensions to the emer.PrjnType types

//go:generate stringer -type=PrjnType

var KiT_PrjnType = kit.Enums.AddEnum(PrjnTypeN, false, nil)

// The DeepLeabra prjn types
const (
	// BurstCtxt are projections from Superficial layers to Deep layers that
	// send DeepBurst activations drive updating of DeepCtxt excitatory conductance,
	// at end of a DeepBurst quarter.  These projections also use a special learning
	// rule that takes into account the temporal delays in the activation states.
	BurstCtxt emer.PrjnType = emer.PrjnTypeN + iota

	// BurstTRC are projections from Superficial layers to TRC (thalamic relay cell)
	// neurons (e.g., in the Pulvinar) that send DeepBurst activation continuously
	// during the DeepBurst quarter(s), driving the TRCBurstGe value, which then drives
	// the 	plus-phase activation state of the TRC representing the "outcome" against
	// which prior predictions are (implicitly) compared via the temporal difference
	// in TRC activation state.
	BurstTRC

	// DeepAttn are projections from Deep layers (representing layer 6 regular-spiking
	// CT corticothalamic neurons) up to corresponding Superficial layer neurons, that drive
	// the attentional modulation of activations there (i.e., DeepAttn and DeepLrn values).
	// This is sent continuously all the time from deep layers using the standard delta-based
	// Ge computation, and aggregated into the AttnGe variable on Super neurons.
	DeepAttn

	PrjnTypeN
)