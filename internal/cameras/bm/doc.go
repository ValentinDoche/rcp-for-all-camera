// Package bm is the shared base of the Blackmagic camera modules. It speaks
// the Blackmagic Camera Control REST API over HTTP (port 80, no
// authentication): writes are PUTs under /control/api/v1, state comes from
// the event WebSocket at /control/api/v1/event/websocket.
//
// bm is not a module. A module (bm/studio6kpro for example) composes it:
// it gives New the controls its camera has (Model.Specs, built with the
// spec helpers of this package) and, when its camera differs from the
// base, hooks that replace one precise behaviour (Model.Hooks).
package bm
