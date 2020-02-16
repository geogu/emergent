// Copyright (c) 2019, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package patgen

import (
	"fmt"
	"log"
	"math"
	"math/rand"

	"github.com/emer/etable/etensor"
	"github.com/emer/etable/tsragg"
	"github.com/goki/ki/ints"
)

// Vocab is a map of named tensors that contain patterns used for creating
// larger patterns by mixing together.
type Vocab map[string]*etensor.Float32

// ByNameTry looks for vocabulary item of given name, and returns
// (and logs) error message if not found
func (vc Vocab) ByNameTry(name string) (*etensor.Float32, error) {
	tsr, ok := vc[name]
	if !ok {
		err := fmt.Errorf("Vocabulary item named: %s not found", name)
		log.Println(err)
		return nil, err
	}
	return tsr, nil
}

// Note: to keep things consistent, all AddVocab functions start with Vocab and name
// args and return the tensor and an error, even if there is no way that they could error.
// Also, all routines should automatically log any error message, and because this is
// "end user" code, it is much better to have error messages instead of crashes
// so we add the extra checks etc.

// NOn returns the number of bits active in given tensor
func NOn(trow *etensor.Float32) int {
	return int(tsragg.Sum(trow))
}

// PctAct returns the percent activity in given tensor (NOn / size)
func PctAct(trow *etensor.Float32) float32 {
	return float32(NOn(trow)) / float32(trow.Len())
}

// AddVocabEmpty adds an empty pool to the vocabulary.
// This can be used to make test cases with missing pools.
func AddVocabEmpty(mp Vocab, name string, rows, poolY, poolX int) (*etensor.Float32, error) {
	tsr := etensor.NewFloat32([]int{rows, poolY, poolX}, nil, []string{"row", "Y", "X"})
	mp[name] = tsr
	return tsr, nil
}

// AddVocabPermutedBinary adds a permuted binary pool to the vocabulary.
// This is a good source of random patterns with no systematic similarity
// (with pctAct percent bits turned on) for a pool.
func AddVocabPermutedBinary(mp Vocab, name string, rows, poolY, poolX int, pctAct float32) (*etensor.Float32, error) {
	nOn := int(math.Round(float64(poolY) * float64(poolX) * float64(pctAct)))
	tsr := etensor.NewFloat32([]int{rows, poolY, poolX}, nil, []string{"row", "Y", "X"})
	PermutedBinaryRows(tsr, nOn, 1, 0)
	mp[name] = tsr
	return tsr, nil
}

// AddVocabClone clones an existing pool in the vocabulary to make a new one.
func AddVocabClone(mp Vocab, name string, copyFrom string) (*etensor.Float32, error) {
	cp, err := mp.ByNameTry(copyFrom)
	if err != nil {
		return nil, err
	}
	tsr := cp.Clone().(*etensor.Float32)
	mp[name] = tsr
	return tsr, nil
}

// AddVocabRepeat adds a repeated pool to the vocabulary,
// copying from given row in existing vocabulary item .
func AddVocabRepeat(mp Vocab, name string, rows int, copyFrom string, copyRow int) (*etensor.Float32, error) {
	cp, err := mp.ByNameTry(copyFrom)
	if err != nil {
		return nil, err
	}
	tsr := &etensor.Float32{}
	cpshp := cp.Shapes()
	cpshp[0] = rows
	tsr.SetShape(cpshp, nil, cp.DimNames())
	mp[name] = tsr
	cprow := cp.SubSpace([]int{copyRow})
	for i := 0; i < rows; i++ {
		trow := tsr.SubSpace([]int{i})
		trow.CopyFrom(cprow)
	}
	return tsr, nil
}

// AddVocabDrift adds a row-by-row drifting pool to the vocabulary,
// starting from the given row in existing vocabulary item
// (which becomes starting row in this one -- drift starts in second row).
// The current row patterns are generated by taking the previous row
// pattern and flipping pctDrift percent of active bits (min of 1 bit).
func AddVocabDrift(mp Vocab, name string, rows int, pctDrift float32, copyFrom string, copyRow int) (*etensor.Float32, error) {
	cp, err := mp.ByNameTry(copyFrom)
	if err != nil {
		return nil, err
	}
	tsr := &etensor.Float32{}
	cpshp := cp.Shapes()
	cpshp[0] = rows
	tsr.SetShape(cpshp, nil, cp.DimNames())
	mp[name] = tsr
	cprow := cp.SubSpace([]int{copyRow}).(*etensor.Float32)
	trow := tsr.SubSpace([]int{0})
	trow.CopyFrom(cprow)
	nOn := NOn(cprow)
	nDrift := int(math.Round(float64(nOn) * float64(pctDrift)))
	nDrift = ints.MaxInt(1, nDrift) // ensure at least one
	for i := 1; i < rows; i++ {
		srow := tsr.SubSpace([]int{i - 1})
		trow := tsr.SubSpace([]int{i})
		trow.CopyFrom(srow)
		FlipBits(trow, nDrift, nDrift, 1, 0)
	}
	return tsr, nil
}

// VocabShuffle shuffles a pool in the vocabulary on its first dimension (row).
func VocabShuffle(mp Vocab, shufflePools []string) {
	for _, key := range shufflePools {
		tsr := mp[key]
		rows := tsr.Shapes()[0]
		poolY := tsr.Shapes()[1]
		poolX := tsr.Shapes()[2]
		sRows := rand.Perm(rows)
		sTsr := etensor.NewFloat32([]int{rows, poolY, poolX}, nil, []string{"row", "Y", "X"})
		for iRow, sRow := range sRows {
			sTsr.SubSpace([]int{iRow}).CopyFrom(tsr.SubSpace([]int{sRow}))
		}
		mp[key] = sTsr
	}
}

// VocabConcat contatenates several pools in the vocabulary and store it into newPool (could be one of the previous pools).
func VocabConcat(mp Vocab, newPool string, frmPools []string) error {
	tsr := mp[frmPools[0]].Clone().(*etensor.Float32)
	for i, key := range frmPools {
		if i > 0 {
			// check pool shape
			if !(tsr.SubSpace([]int{0}).(*etensor.Float32).Shape.IsEqual(&mp[key].SubSpace([]int{0}).(*etensor.Float32).Shape)) {
				err := fmt.Errorf("shapes of input pools must be the same") // how do I stop the program?
				log.Println(err.Error())
				return err
			}

			currows := tsr.Shapes()[0]
			approws := mp[key].Shapes()[0]
			tsr.SetShape([]int{currows + approws, tsr.Shapes()[1], tsr.Shapes()[2]}, nil, []string{"row", "Y", "X"})
			for iRow := 0; iRow < approws; iRow++ {
				subtsr := tsr.SubSpace([]int{iRow + currows})
				subtsr.CopyFrom(mp[key].SubSpace([]int{iRow}))
			}
		}
	}
	mp[newPool] = tsr
	return nil
}

// VocabSlice slices a pool in the vocabulary into new ones.
// SliceOffs is the cutoff points in the original pool, should have one more element than newPools.
func VocabSlice(mp Vocab, frmPool string, newPools []string, sliceOffs []int) error {
	oriTsr := mp[frmPool]
	poolY := oriTsr.Shapes()[1]
	poolX := oriTsr.Shapes()[2]

	// check newPools and sliceOffs have same length
	if len(newPools)+1 != len(sliceOffs) {
		err := fmt.Errorf("sliceOffs should have one more element than newPools") // how do I stop the program?
		log.Println(err.Error())
		return err
	}

	// check sliceOffs is in right order
	preVal := sliceOffs[0]
	for i, curVal := range sliceOffs {
		if i > 0 {
			if preVal < curVal {
				preVal = curVal
			} else {
				err := fmt.Errorf("sliceOffs should increase progressively") // how do I stop the program?
				log.Println(err.Error())
				return err
			}
		}
	}

	// slice
	frmOff := sliceOffs[0]
	for i := range newPools {
		toOff := sliceOffs[i+1]
		newPool := newPools[i]
		newTsr := etensor.NewFloat32([]int{toOff - frmOff, poolY, poolX}, nil, []string{"row", "Y", "X"})
		for off := frmOff; off < toOff; off++ {
			newTsr.SubSpace([]int{off - frmOff}).CopyFrom(oriTsr.SubSpace([]int{off}))
		}
		mp[newPool] = newTsr
		frmOff = toOff
	}
	return nil
}