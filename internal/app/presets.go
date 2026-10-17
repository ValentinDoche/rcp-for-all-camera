package app

import (
	"context"
	"fmt"
	"maps"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Presets are snapshots of a camera's control values, kept by the app in
// the rig file and recalled by writing the values back. They are never
// stored on the camera, so they work the same for every driver.

// slotPresets holds the presets of one slot; index n-1 is preset n. A
// stored *SavedPreset is never modified in place (it is replaced), so
// copying the array is enough to roll a change back.
type slotPresets [PresetCount]*SavedPreset

// numbers returns the stored preset numbers, sorted, never nil.
func (p slotPresets) numbers() []int {
	out := []int{}
	for i, pr := range p {
		if pr != nil {
			out = append(out, i+1)
		}
	}
	return out
}

// storable reports whether a control is part of a preset: a value the
// operator sets, that is safe to write back. Read-only and action controls
// have nothing to restore, guarded ones change recording or what goes on
// air, and PTZ controls are speeds or positions that would move the camera.
func storable(sp core.ControlSpec) bool {
	return !sp.ReadOnly && !sp.Hidden && sp.Kind != core.KindAction && !sp.Guarded && sp.Group != core.GroupPTZ
}

func checkPreset(n int) error {
	if n < 1 || n > PresetCount {
		return &Error{Code: CodeInvalidPreset, Message: fmt.Sprintf("preset must be 1 to %d", PresetCount)}
	}
	return nil
}

func notConnected(slot int) error {
	return &Error{Code: CodeNotConnected, Message: fmt.Sprintf("camera %d is not connected", slot)}
}

// SetStoreArmed arms or disarms "Store": while armed, the next preset number
// pressed (panel scene key or GUI number) stores instead of recalling. The
// state is global (one operator) and shared by the panel and the GUI.
// EventStoreArmed is emitted only when the state changes.
func (s *Service) SetStoreArmed(on bool) {
	s.mu.Lock()
	if s.storeArmed == on {
		s.mu.Unlock()
		return
	}
	s.storeArmed = on
	emit := s.emit
	s.mu.Unlock()
	emit.Emit(EventStoreArmed, StoreArmedState{Armed: on})
}

// StoreArmed reports whether Store is armed.
func (s *Service) StoreArmed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.storeArmed
}

// StorePreset snapshots the storable controls of the slot's camera as
// preset n, replacing any preset already stored there. Whatever the
// outcome, Store is disarmed afterwards: the gesture is over, and a failed
// store must not leave the next number press storing by surprise.
func (s *Service) StorePreset(slot, n int) error {
	defer s.SetStoreArmed(false)
	if err := checkSlot(slot); err != nil {
		return err
	}
	if err := checkPreset(n); err != nil {
		return err
	}
	cam, err := s.camera(slot)
	if err != nil {
		return err
	}
	if cam.Status().State != core.StateConnected {
		return notConnected(slot)
	}
	// Read outside the lock, with one deadline for the whole snapshot.
	values := map[core.ControlID]core.Value{}
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	for _, sp := range cam.Controls() {
		if !storable(sp) || ctx.Err() != nil {
			continue
		}
		v, err := cam.Get(ctx, sp.ID)
		if err != nil {
			s.log.Debug("preset: control not read", "slot", slot, "control", sp.ID, "err", err)
			continue
		}
		values[sp.ID] = v
	}
	cancel()
	if len(values) == 0 {
		return notConnected(slot)
	}

	s.mu.Lock()
	st := s.slots[slot-1]
	if st == nil || st.cam != cam {
		// The slot was cleared or reassigned while reading.
		s.mu.Unlock()
		return &Error{Code: CodeEmptySlot, Message: fmt.Sprintf("slot %d changed while storing the preset", slot)}
	}
	prev := s.presets[slot-1][n-1]
	s.presets[slot-1][n-1] = &SavedPreset{Slot: slot, Number: n, Model: st.info.ID, Values: values}
	if err := s.saveLocked(); err != nil {
		s.presets[slot-1][n-1] = prev
		s.mu.Unlock()
		return toAPIError(err)
	}
	rig, emit := s.rigLocked(), s.emit
	s.mu.Unlock()
	s.log.Info("preset stored", "slot", slot, "preset", n, "controls", len(values))
	emit.Emit(EventRigChanged, rig)
	return nil
}

// RecallPreset writes preset n back to the slot's camera, in the camera's
// control order. A control that fails is skipped and the others are still
// written; the first error is returned once all were tried.
func (s *Service) RecallPreset(slot, n int) error {
	if err := checkSlot(slot); err != nil {
		return err
	}
	if err := checkPreset(n); err != nil {
		return err
	}
	s.mu.Lock()
	st := s.slots[slot-1]
	if st == nil {
		s.mu.Unlock()
		return &Error{Code: CodeEmptySlot, Message: fmt.Sprintf("slot %d has no camera", slot)}
	}
	p := s.presets[slot-1][n-1]
	if p == nil {
		s.mu.Unlock()
		return &Error{Code: CodeEmptyPreset, Message: fmt.Sprintf("no preset %d on slot %d", n, slot)}
	}
	if p.Model != st.info.ID {
		s.mu.Unlock()
		return &Error{Code: CodeInvalidPreset, Message: "the preset was stored on another camera model"}
	}
	cam, values := st.cam, maps.Clone(p.Values)
	s.mu.Unlock()

	if cam.Status().State != core.StateConnected {
		return notConnected(slot)
	}
	var first error
	written := 0
	for _, sp := range cam.Controls() {
		v, ok := values[sp.ID]
		if !ok || !storable(sp) {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
		s.log.Debug("preset: set control", "slot", slot, "preset", n, "control", sp.ID, "value", v)
		err := cam.Set(ctx, sp.ID, v)
		cancel()
		if err != nil {
			s.log.Debug("preset: control not written", "slot", slot, "control", sp.ID, "err", err)
			if first == nil {
				first = err
				s.commandFailed(slot, sp.ID, toAPIError(err))
			}
			continue
		}
		written++
	}
	s.log.Info("preset recalled", "slot", slot, "preset", n, "controls", written, "failed", first != nil)
	return toAPIError(first)
}

// DeletePreset removes preset n of slot. Deleting a missing preset succeeds.
func (s *Service) DeletePreset(slot, n int) error {
	if err := checkSlot(slot); err != nil {
		return err
	}
	if err := checkPreset(n); err != nil {
		return err
	}
	s.mu.Lock()
	prev := s.presets[slot-1][n-1]
	if prev == nil {
		s.mu.Unlock()
		return nil
	}
	s.presets[slot-1][n-1] = nil
	if err := s.saveLocked(); err != nil {
		s.presets[slot-1][n-1] = prev
		s.mu.Unlock()
		return toAPIError(err)
	}
	rig, emit := s.rigLocked(), s.emit
	s.mu.Unlock()
	s.log.Info("preset deleted", "slot", slot, "preset", n)
	emit.Emit(EventRigChanged, rig)
	return nil
}

// savedPresetsLocked returns the presets to persist, in slot then number
// order, with their own copy of the values.
func (s *Service) savedPresetsLocked() []SavedPreset {
	var out []SavedPreset
	for _, sp := range s.presets {
		for _, p := range sp {
			if p != nil {
				c := *p
				c.Values = maps.Clone(p.Values)
				out = append(out, c)
			}
		}
	}
	return out
}

// loadPresets keeps the saved presets that still match the loaded slots.
// Called by NewService after the slots are loaded; the rest is dropped
// with a warning, like invalid cameras.
func (s *Service) loadPresets(saved []SavedPreset) {
	for _, p := range saved {
		drop := func(reason string) {
			s.log.Warn("dropping saved preset", "slot", p.Slot, "preset", p.Number, "reason", reason)
		}
		if p.Slot < 1 || p.Slot > SlotCount {
			drop("slot out of range")
			continue
		}
		if p.Number < 1 || p.Number > PresetCount {
			drop("number out of range")
			continue
		}
		st := s.slots[p.Slot-1]
		if st == nil {
			drop("slot has no camera")
			continue
		}
		if p.Model != st.info.ID {
			drop("taken on another camera model")
			continue
		}
		if s.presets[p.Slot-1][p.Number-1] != nil {
			drop("duplicate, keeping the last one")
		}
		values := make(map[core.ControlID]core.Value, len(p.Values))
		for id, v := range p.Values {
			if core.CheckControlID(id) != nil || v.Check() != nil {
				s.log.Warn("dropping invalid preset value", "slot", p.Slot, "preset", p.Number)
				continue
			}
			values[id] = v
		}
		s.presets[p.Slot-1][p.Number-1] = &SavedPreset{Slot: p.Slot, Number: p.Number, Model: p.Model, Values: values}
	}
}
