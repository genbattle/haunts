package actions_test

import (
  . "gospec"
  "gospec"
  "path/filepath"
  "encoding/gob"
  "bytes"
  "haunts/base"
  "haunts/game"
  "haunts/game/actions"
)

var datadir string

func init() {
  datadir,_ = filepath.Abs("../../data_test")
  base.SetDatadir(datadir)
}

func ActionSpec(c gospec.Context) {
  game.RegisterActions()
  c.Specify("Actions are loaded properly.", func() {
    basic := game.MakeAction("Basic Test")
    _,ok := basic.(*actions.BasicAttack)
    c.Expect(ok, Equals, true)

    charge := game.MakeAction("Charge Test")
    _,ok = charge.(*actions.ChargeAttack)
    c.Expect(ok, Equals, true)
  })

  c.Specify("Actions can be gobbed without loss of type.", func() {
    buf := bytes.NewBuffer(nil)
    enc := gob.NewEncoder(buf)

    var as []game.Action
    as = append(as, game.MakeAction("Move Test"))
    as = append(as, game.MakeAction("Basic Test"))
    as = append(as, game.MakeAction("Charge Test"))

    err := enc.Encode(as)
    c.Assume(err, Equals, nil)

    dec := gob.NewDecoder(buf)
    var as2 []game.Action
    err = dec.Decode(&as2)
    c.Assume(err, Equals, nil)

    _,ok := as2[0].(*actions.Move)
    c.Expect(ok, Equals, true)

    _,ok = as2[1].(*actions.BasicAttack)
    c.Expect(ok, Equals, true)

    _,ok = as2[2].(*actions.ChargeAttack)
    c.Expect(ok, Equals, true)

  })
}
