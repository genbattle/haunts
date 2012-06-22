package game

import (
  "math/rand"
  "path/filepath"
  "io/ioutil"
  "regexp"
  "github.com/runningwild/glop/gui"
  "github.com/runningwild/haunts/base"
  "github.com/runningwild/haunts/house"
  lua "github.com/xenith-studios/golua"
)

type gameScript struct {
  L *lua.State

  // Since the scripts can do anything they want sometimes we want make sure
  // certain things only run when the game is ready for them.
  sync chan struct{}
}

func (gs *gameScript) syncStart() {
  <-gs.sync
}
func (gs *gameScript) syncEnd() {
  gs.sync <- struct{}{}
}

func startGameScript(gp *GamePanel, path string) {
  // Clear out the panel, now the script can do whatever it wants
  gp.AnchorBox = gui.MakeAnchorBox(gui.Dims{1024,700})
  base.Log().Printf("startGameScript")
  if !filepath.IsAbs(path) {
    path = filepath.Join(base.GetDataDir(), "scripts", path)
  }

  // The game script runs in a separate go routine and functions that need to
  // communicate with the game will do so via channels - DUH why did i even
  // write this comment?
  prog, err := ioutil.ReadFile(path)
  if err != nil {
    base.Error().Printf("Unable to load game script file %s: %v", path, err)
    return
  }
  gp.script = &gameScript{}
  gp.script.L = lua.NewState()
  gp.script.L.OpenLibs()
  gp.script.L.SetExecutionLimit(25000)
  registerUtilityFunctions(gp.script.L)
  gp.script.L.Register("loadHouse", loadHouse(gp))
  gp.script.L.Register("showMainBar", showMainBar(gp))
  gp.script.L.Register("spawnEntityAtPosition", spawnEntityAtPosition(gp))
  gp.script.L.Register("getSpawnPointsMatching", getSpawnPointsMatching(gp))
  gp.script.L.Register("spawnEntitySomewhereInSpawnPoints", spawnEntitySomewhereInSpawnPoints(gp))
  gp.script.L.Register("placeEntities", placeEntities(gp))
  gp.script.L.Register("roomAtPos", roomAtPos(gp))
  gp.script.L.Register("setLosMode", setLosMode(gp))
  gp.script.L.Register("getAllEnts", getAllEnts(gp))
  gp.script.L.Register("selectMap", selectMap(gp))

  gp.script.sync = make(chan struct{})
  res := gp.script.L.DoString(string(prog))
  if !res {
    base.Error().Printf("There was an error running script %s:\n%s", path, prog)
  } else {
    go func() {
      gp.script.L.SetExecutionLimit(250000)
      gp.script.L.GetField(lua.LUA_GLOBALSINDEX, "Init")
      gp.script.L.Call(0, 0)
      gp.game.comm.script_to_game <- nil
    } ()
  }
}

// Runs RoundStart
// Lets the game know that the round middle can begin
// Runs RoundEnd
func (gs *gameScript) OnRound(g *Game) {
  base.Log().Printf("Launching script.OnRound")
  go func() {
    // // round begins automatically
    // <-round_middle
    // for
    //   <-action stuff
    // <- round end
    // <- round end done
    gs.L.SetExecutionLimit(250000)
    gs.L.GetField(lua.LUA_GLOBALSINDEX, "RoundStart")
    gs.L.PushBoolean(g.Side == SideExplorers)
    gs.L.PushInteger((g.Turn + 1) / 2)
    gs.L.Call(2, 0)

    // signals to the game that we're done with the startup stuff
    g.comm.script_to_game <- nil
    base.Log().Printf("ScriptComm: Done with OnStart")

    for {
      // The action is sent when it happens, and a nil is sent when it is done
      // being executed, we want to wait until then so that the game is in a
      // stable state before we do anything.
      base.Log().Printf("ScriptComm: Waiting for action")
      action := <-g.comm.game_to_script
      base.Log().Printf("ScriptComm: Got action: %v", action)
      if action == nil {
        base.Log().Printf("ScriptComm: No more action: bailing")
        break
      }
      <-g.comm.game_to_script
    base.Log().Printf("ScriptComm: Got action secondary")
      // Run OnAction here
      g.comm.script_to_game <- nil
    base.Log().Printf("ScriptComm: Done with OnAction")
    }

    gs.L.SetExecutionLimit(250000)
    gs.L.GetField(lua.LUA_GLOBALSINDEX, "RoundEnd")
    gs.L.PushBoolean(g.Side == SideExplorers)
    gs.L.PushInteger((g.Turn + 1) / 2)
    gs.L.Call(2, 0)

    // Signal that we're done with the round end
    g.comm.script_to_game <- nil
    base.Log().Printf("ScriptComm: Done with RoundEnd")
  } ()
}

// Can be called occassionally and will allow a script to progress whenever
// it is ready
func (gp *GamePanel) scriptThinkOnce() {
  if gp.script.L == nil {
    return
  }
  done := false
  for !done {
    select {
    // If a script has tried to run a function that requires running during
    // Think then it can run now and we'll wait for it to finish before
    // continuing.
    case gp.script.sync <- struct{}{}:
      <-gp.script.sync
    default:
      done = true
    }
  }
}

// Thinks continually until a value is passed along done
func (gp *GamePanel) scriptSitAndThink() (done chan<- struct{}) {
  done_chan := make(chan struct{})

  go func() {
    for {
      select {
      case <-gp.script.sync:
        <-gp.script.sync
      case <-done_chan:
        return
      }
    }
  } ()

  return done_chan
}

func loadHouse(gp *GamePanel) lua.GoFunction {
  return func(L* lua.State) int {
    gp.script.syncStart()
    defer gp.script.syncEnd()

    name := L.ToString(-1)
    def := house.MakeHouseFromName(name)
    if def == nil || len(def.Floors) == 0 {
      base.Error().Printf("No house exists with the name '%s'.", name)
      return 0
    }
    gp.house = def
    gp.viewer = house.MakeHouseViewer(gp.house, 62)
    gp.viewer.Edit_mode = true
    gp.game = makeGame(gp.house, gp.viewer, SideExplorers)
    gp.game.script = gp.script

    gp.AnchorBox = gui.MakeAnchorBox(gui.Dims{1024,700})

    gp.AnchorBox.AddChild(gp.viewer, gui.Anchor{0.5,0.5,0.5,0.5})
    base.Log().Printf("Done making stuff")
    return 0
  }
}

func showMainBar(gp *GamePanel) lua.GoFunction {
  return func(L *lua.State) int {
    gp.script.syncStart()
    defer gp.script.syncEnd()
    show := L.ToBoolean(-1)

    // Remove it regardless of whether or not we want to hide it
    for _, child := range gp.AnchorBox.GetChildren() {
      if child == gp.main_bar {
        gp.AnchorBox.RemoveChild(child)
        break
      }
    }

    if show {
      var err error
      gp.main_bar,err = MakeMainBar(gp.game)
      if err != nil {
        base.Error().Printf("%v", err)
        return 0
      }
      gp.AnchorBox.AddChild(gp.main_bar, gui.Anchor{0.5,0,0.5,0})
    }
    base.Log().Printf("Num kids: %d", len(gp.AnchorBox.GetChildren()))
    return 0
  }
}

func spawnEntityAtPosition(gp *GamePanel) lua.GoFunction {
  return func(L *lua.State) int {
    gp.script.syncStart()
    defer gp.script.syncEnd()
    name := L.ToString(-3)
    x := L.ToInteger(-2)
    y := L.ToInteger(-1)
    gp.game.SpawnEntity(MakeEntity(name, gp.game), x, y)
    return 0
  }
}

func getSpawnPointsMatching(gp *GamePanel) lua.GoFunction {
  return func(L *lua.State) int {
    gp.script.syncStart()
    defer gp.script.syncEnd()
    spawn_pattern := L.ToString(-1)
    re, err := regexp.Compile(spawn_pattern)
    if err != nil {
      base.Error().Printf("Failed to compile regexp '%s': %v", spawn_pattern, err)
      return 0
    }
    L.NewTable()
    count := 0
    for index, sp := range gp.game.House.Floors[0].Spawns {
      if !re.MatchString(sp.Name) {
        continue
      }
      count++
      L.PushInteger(count)
      L.NewTable()
      x, y := sp.Pos()
      dx, dy := sp.Dims()
      L.PushString("id")
      L.PushInteger(index)
      L.SetTable(-3)
      L.PushString("name")
      L.PushString(sp.Name)
      L.SetTable(-3)
      L.PushString("x")
      L.PushInteger(x)
      L.SetTable(-3)
      L.PushString("y")
      L.PushInteger(y)
      L.SetTable(-3)
      L.PushString("dx")
      L.PushInteger(dx)
      L.SetTable(-3)
      L.PushString("dy")
      L.PushInteger(dy)
      L.SetTable(-3)
      L.SetTable(-3)
    }
    return 1
  }
}

func spawnEntitySomewhereInSpawnPoints(gp *GamePanel) lua.GoFunction {
  return func(L *lua.State) int {
    gp.script.syncStart()
    defer gp.script.syncEnd()
    name := L.ToString(-2)

    var tx,ty int
    count := 0
    sp_count := 1
    L.PushInteger(sp_count)
    L.GetTable(-2)
    for !L.IsNil(-1) {
      L.PushString("id")
      L.GetTable(-2)
      id := L.ToInteger(-1)
      L.Pop(2)
      sp_count++
      L.PushInteger(sp_count)
      L.GetTable(-2)
      if id < 0 || id >= len(gp.game.House.Floors[0].Spawns) {
        base.Error().Printf("Tried to access an unknown spawn point: %d", id)
        break
      }
      sp := gp.game.House.Floors[0].Spawns[id]
      sx, sy := sp.Pos()
      sdx, sdy := sp.Dims()
      for x := sx; x < sx + sdx; x++ {
        for y := sy; y < sy + sdy; y++ {
          if gp.game.IsCellOccupied(x, y) {
            continue
          }
          // This will choose a random position from all positions and giving
          // all positions an equal chance of being chosen.
          count++
          if rand.Intn(count) == 0 {
            tx = x
            ty = y
          }
        }
      }
    }
    if count == 0 {
      base.Error().Printf("Unable to find an available position to spawn")
      return 0
    }
    ent := MakeEntity(name, gp.game)
    if ent == nil {
      base.Error().Printf("Cannot make an entity named '%s', no such thing.", name)
      return 0
    }
    gp.game.SpawnEntity(ent, tx, ty)

    L.NewTable()
    L.PushString("x")
    L.PushInteger(tx)
    L.SetTable(-3)
    L.PushString("y")
    L.PushInteger(ty)
    L.SetTable(-3)
    return 1
  }
}



func placeEntities(gp *GamePanel) lua.GoFunction {
  return func(L *lua.State) int {
    gp.script.syncStart()
    pattern := L.ToString(-3)
    points := L.ToInteger(-2)

    var names []string
    var costs []int
    L.PushNil()
    for L.Next(-2) != 0 {
      L.PushInteger(1)
      L.GetTable(-2)
      names = append(names, L.ToString(-1))
      L.Pop(1)
      L.PushInteger(2)
      L.GetTable(-2)
      costs = append(costs, L.ToInteger(-1))
      L.Pop(1)
      L.Pop(1)
    }

    ep, placed_chan := MakeEntityPlacer(gp.game, pattern, points, names, costs)
    gp.AnchorBox.AddChild(ep, gui.Anchor{0.5,0.5,0.5,0.5})
    gp.script.syncEnd()

    placed := <-placed_chan
    L.NewTable()
    for i := range placed {
      L.PushInteger(i + 1)
      pushEntity(L, placed[i])
      L.SetTable(-3)
    }

    gp.script.syncStart()
    gp.AnchorBox.RemoveChild(ep)
    gp.script.syncEnd()
    return 1
  }
}

func pushPoint(L *lua.State, x, y int) {
  L.NewTable()
  L.PushString("x")
  L.PushInteger(x)
  L.SetTable(-3)
  L.PushString("y")
  L.PushInteger(y)
  L.SetTable(-3)
}

func toPoint(L *lua.State, pos int) (x, y int) {
  L.PushString("x")
  L.GetTable(pos - 1)
  x = L.ToInteger(-1)
  L.Pop(1)
  L.PushString("y")
  L.GetTable(pos - 1)
  y = L.ToInteger(-1)
  L.Pop(1)
  return
}

func pushEntity(L *lua.State, ent *Entity) {
  L.NewTable()
  L.PushString("id")
  L.PushInteger(int(ent.Id))
  L.SetTable(-3)
  L.PushString("name")
  L.PushString(ent.Name)
  L.SetTable(-3)
  L.PushString("pos")
  x, y := ent.Pos()
  pushPoint(L, x, y)
  L.SetTable(-3)
  L.PushString("corpus")
  L.PushInteger(ent.Stats.Corpus())
  L.SetTable(-3)
  L.PushString("ego")
  L.PushInteger(ent.Stats.Ego())
  L.SetTable(-3)
  L.PushString("hpCur")
  L.PushInteger(ent.Stats.HpCur())
  L.SetTable(-3)
  L.PushString("hpMax")
  L.PushInteger(ent.Stats.HpMax())
  L.SetTable(-3)
  L.PushString("apCur")
  L.PushInteger(ent.Stats.ApCur())
  L.SetTable(-3)
  L.PushString("apMax")
  L.PushInteger(ent.Stats.ApMax())
  L.SetTable(-3)
}

func getAllEnts(gp *GamePanel) lua.GoFunction {
  return func(L *lua.State) int {
    gp.script.syncStart()
    defer gp.script.syncEnd()
    L.NewTable()
    for i := range gp.game.Ents {
      L.PushInteger(i+1)
      pushEntity(L, gp.game.Ents[i])
      L.SetTable(-3)
    }
    return 1
  }
}

func roomAtPos(gp *GamePanel) lua.GoFunction {
  return func(L *lua.State) int {
    gp.script.syncStart()
    defer gp.script.syncEnd()
    x, y := toPoint(L, -1)
    room, _, _ := gp.game.House.Floors[0].RoomFurnSpawnAtPos(x, y)
    for i, r := range gp.game.House.Floors[0].Rooms {
      if r == room {
        L.PushInteger(i)
        return 1
      }
    }
    base.Error().Printf("Tried to get the room at position (%d,%d), but there is no room there.", x, y)
    return 0
  }
}

func setLosMode(gp *GamePanel) lua.GoFunction {
  return func(L *lua.State) int {
    gp.script.syncStart()
    defer gp.script.syncEnd()
    mode_str := L.ToString(1)
    switch mode_str {
    case "none":
      gp.game.SetLosMode(LosModeNone, nil)
    case "all":
      gp.game.SetLosMode(LosModeAll, nil)
    case "entities":
      gp.game.SetLosMode(LosModeEntities, nil)
    case "rooms":
      if !L.IsTable(-1) {
        base.Error().Printf("The second parameter to setLosMode should be an array of rooms if mode == 'rooms'")
        return 0
      }
      L.PushNil()
      all_rooms := gp.game.House.Floors[0].Rooms
      var rooms []*house.Room
      for L.Next(-2) != 0 {
        index := L.ToInteger(-1)
        if index < 0 || index > len(all_rooms) {
          base.Error().Printf("Tried to reference room #%d which doesn't exist.", index)
          continue
        }
        rooms = append(rooms, all_rooms[index])
        L.Pop(1)
      }
      gp.game.SetLosMode(LosModeRooms, rooms)

    default:
      base.Error().Printf("Unknown los mode '%s'", mode_str)
      return 0
    }
    return 0
  }
}

func selectMap(gp *GamePanel) lua.GoFunction {
  return func(L *lua.State) int {
    gp.script.syncStart()
    selector, output, err := MakeUiSelectMap(gp)
    if err != nil {
      base.Error().Printf("Error selecting map: %v", err)
      return 0
    }
    gp.AnchorBox.AddChild(selector, gui.Anchor{0.5,0.5,0.5,0.5})
    gp.script.syncEnd()

    name := <-output

    gp.script.syncStart()
    gp.AnchorBox.RemoveChild(selector)
    L.PushString(name)
    gp.script.syncEnd()
    return 1
  }
}

// Ripped from game/ai/ai.go - should probably sync up with it
func registerUtilityFunctions(L *lua.State) {
  L.Register("print", func(L *lua.State) int {
    var res string
    n := L.GetTop()
    for i := -n; i < 0; i++ {
      res += luaStringifyParam(L, i) + " "
    }
    base.Log().Printf("GameScript(%p): %s", L, res)
    return 0
  })
}

func luaStringifyParam(L *lua.State, index int) string {
  if L.IsTable(index) {
    return "table"
  }
  if L.IsBoolean(index) {
    if L.ToBoolean(index) {
      return "true"
    }
    return "false"
  }
  return L.ToString(index)
}