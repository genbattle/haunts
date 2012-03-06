package game

import (
  "fmt"
  "path/filepath"
  "github.com/runningwild/glop/gin"
  "github.com/runningwild/glop/gui"
  "github.com/runningwild/haunts/base"
  "github.com/runningwild/haunts/texture"
  "github.com/runningwild/opengl/gl"
)

type Button struct {
  X,Y int
  Texture texture.Object

  // Color - brighter when the mouse is over it
  shade float64

  // Function to run whenever the button is clicked
  f func(*MainBar)
}

// If x,y is inside the button's region then it will run its function and
// return true, otherwise it does nothing and returns false.
func (b *Button) handleClick(x,y int, mb *MainBar) bool {
  d := b.Texture.Data()
  if x < b.X || y < b.Y || x >= b.X + d.Dx || y >= b.Y + d.Dy {
    return false
  }
  b.f(mb)
  return true
}

func (b *Button) RenderAt(x,y,mx,my int) {
  b.Texture.Data().Bind()
  tdx :=  + b.Texture.Data().Dx
  tdy :=  + b.Texture.Data().Dy
  if mx >= x + b.X && mx < x + b.X + tdx && my >= y + b.Y && my < y + b.Y + tdy {
    b.shade = b.shade * 0.9 + 0.1
  } else {
    b.shade = b.shade * 0.9 + 0.04
  }
  gl.Color4d(1, 1, 1, b.shade)
  gl.Begin(gl.QUADS)
    gl.TexCoord2d(0, 0)
    gl.Vertex2i(x + b.X, y + b.Y)

    gl.TexCoord2d(0, -1)
    gl.Vertex2i(x + b.X, y + b.Y + tdy)

    gl.TexCoord2d(1,-1)
    gl.Vertex2i(x + b.X + tdx, y + b.Y + tdy)

    gl.TexCoord2d(1, 0)
    gl.Vertex2i(x + b.X + tdx, y + b.Y)
  gl.End()
}

type Center struct {
  X,Y int
}

type TextArea struct {
  X,Y           int
  Size          int
  Justification string
}

func (t *TextArea) RenderString(s string) {
  var just gui.Justification
  switch t.Justification {
  case "center":
    just = gui.Center
  case "left":
    just = gui.Left
  case "right":
    just = gui.Right
  default:
    base.Warn().Printf("Unknown justification '%s' in main gui bar.", t.Justification)
    t.Justification = "center"
  }
  px := float64(t.X)
  py := float64(t.Y)
  d := base.GetDictionary(t.Size)
  d.RenderString(s, px, py, 0, d.MaxHeight(), just)
}

type MainBarLayout struct {
  EndTurn     Button
  UnitLeft    Button
  UnitRight   Button
  ActionLeft  Button
  ActionRight Button

  CenterStillFrame Center

  Background texture.Object
  Divider    texture.Object
  Name   TextArea
  Ap     TextArea
  Hp     TextArea
  Corpus TextArea
  Ego    TextArea

  Conditions struct {
    X,Y,Height,Width,Size,Spacing float64
  }

  Actions struct {
    X,Y,Width,Icon_size float64
    Count int
    Empty texture.Object
  }
}

type mainBarState struct {
  Actions struct {
    // target is the action that should be displayed as left-most,
    // pos is the action that is currently left-most, which can be fractional.
    scroll_target float64
    scroll_pos    float64

    selected Action
  }
}

type MainBar struct {
  layout MainBarLayout
  state  mainBarState
  region gui.Region

  // List of all buttons, just to make it easy to iterate through them.
  buttons []*Button

  ent *Entity

  game *Game

  // Position of the mouse
  mx,my int
}

func buttonFuncEndTurn(mb *MainBar) {
  mb.game.OnRound()
}
func buttonFuncActionLeft(mb *MainBar) {
  if mb.ent == nil {
    return
  }
  start_index := len(mb.ent.Actions)-1
  if mb.state.Actions.selected != nil {
    for i := range mb.ent.Actions {
      if mb.ent.Actions[i] == mb.state.Actions.selected {
        start_index = i-1
        break
      }
    }
  }
  for i := start_index; i >= 0; i-- {
    action := mb.ent.Actions[i]
    if action.Preppable(mb.ent, mb.game) {
      action.Prep(mb.ent, mb.game)
      mb.game.SetCurrentAction(action)
      return
    }
  }
}
func buttonFuncActionRight(mb *MainBar) {
  if mb.ent == nil {
    return
  }
  start_index := 0
  if mb.state.Actions.selected != nil {
    for i := range mb.ent.Actions {
      if mb.ent.Actions[i] == mb.state.Actions.selected {
        start_index = i+1
        break
      }
    }
  }
  for i := start_index; i < len(mb.ent.Actions); i++ {
    action := mb.ent.Actions[i]
    if action.Preppable(mb.ent, mb.game) {
      action.Prep(mb.ent, mb.game)
      mb.game.SetCurrentAction(action)
      return
    }
  }
}
func buttonFuncUnitLeft(mb *MainBar) {
  mb.game.SetCurrentAction(nil)
  start_index := len(mb.game.Ents) - 1
  for i := 0; i < len(mb.game.Ents); i++ {
    if mb.game.Ents[i] == mb.ent {
      start_index = i
      break
    }
  }
  for i := start_index - 1; i >= 0; i-- {
    if mb.game.Ents[i].Side == mb.game.Side {
      mb.game.selected_ent = mb.game.Ents[i]
      return
    }
  }
  for i := len(mb.game.Ents) - 1; i >= start_index; i-- {
    if mb.game.Ents[i].Side == mb.game.Side {
      mb.game.selected_ent = mb.game.Ents[i]
      return
    }
  }
}
func buttonFuncUnitRight(mb *MainBar) {
  mb.game.SetCurrentAction(nil)
  start_index := 0
  for i := 0; i < len(mb.game.Ents); i++ {
    if mb.game.Ents[i] == mb.ent {
      start_index = i
      break
    }
  }
  for i := start_index + 1; i < len(mb.game.Ents); i++ {
    if mb.game.Ents[i].Side == mb.game.Side {
      mb.game.selected_ent = mb.game.Ents[i]
      return
    }
  }
  for i := 0; i <= start_index; i++ {
    if mb.game.Ents[i].Side == mb.game.Side {
      mb.game.selected_ent = mb.game.Ents[i]
      return
    }
  }
}

func MakeMainBar(game *Game) (*MainBar, error) {
  var mb MainBar
  datadir := base.GetDataDir()
  err := base.LoadAndProcessObject(filepath.Join(datadir, "ui", "main_bar.json"), "json", &mb.layout)
  if err != nil {
    return nil, err
  }
  mb.buttons = []*Button{
    &mb.layout.EndTurn,
    &mb.layout.UnitLeft,
    &mb.layout.UnitRight,
    &mb.layout.ActionLeft,
    &mb.layout.ActionRight,
  }
  mb.layout.EndTurn.f = buttonFuncEndTurn
  mb.layout.UnitLeft.f = buttonFuncUnitLeft
  mb.layout.UnitRight.f = buttonFuncUnitRight
  mb.layout.ActionLeft.f = buttonFuncActionLeft
  mb.layout.ActionRight.f = buttonFuncActionRight
  mb.game = game
  return &mb, nil
}
func (m *MainBar) Requested() gui.Dims {
  return gui.Dims{
    Dx: m.layout.Background.Data().Dx,
    Dy: m.layout.Background.Data().Dy,
  }
}

func (mb *MainBar) SelectEnt(ent *Entity) {
  if ent == mb.ent {
    return
  }
  mb.ent = ent
  mb.state = mainBarState{}

  if mb.ent == nil {
    return
  }
  mb.state.Actions.selected = mb.ent.selected_action
}

func (m *MainBar) Expandable() (bool, bool) {
  return false, false
}

func (m *MainBar) Rendered() gui.Region {
  return m.region
}

func (m *MainBar) Think(g *gui.Gui, t int64) {
  if m.ent != nil {
    min := 0.0
    max := float64(len(m.ent.Actions) - m.layout.Actions.Count)
    selected_index := -1
    for i := range m.ent.Actions {
      if m.ent.Actions[i] == m.state.Actions.selected {
        selected_index = i
        break
      }
    }
    if selected_index != -1 {
      if min < float64(selected_index - m.layout.Actions.Count + 1) {
        min = float64(selected_index - m.layout.Actions.Count + 1)
      }
      if max > float64(selected_index) {
        max = float64(selected_index)
      }
    }
    m.state.Actions.selected = m.ent.selected_action
    if m.state.Actions.scroll_target > max {
      m.state.Actions.scroll_target = max
    }
    if m.state.Actions.scroll_target < min {
      m.state.Actions.scroll_target = min
    }

    // If an action is selected and we can't see it then we scroll just enough
    // so that we can.

  } else {
    m.state.Actions.scroll_pos = 0
    m.state.Actions.scroll_target = 0
  }

  // Do a nice scroll motion towards the target position
  m.state.Actions.scroll_pos *= 0.8
  m.state.Actions.scroll_pos += 0.2 * m.state.Actions.scroll_target
}

func (m *MainBar) Respond(g *gui.Gui, group gui.EventGroup) bool {
  cursor := group.Events[0].Key.Cursor()
  if cursor != nil {
    m.mx, m.my = cursor.Point()
  }

  if found, event := group.FindEvent(gin.MouseLButton); found && event.Type == gin.Press {
    for _, button := range m.buttons {
      if button.handleClick(m.mx, m.my, m) {
        return true
      }
    }
  }

  return false
}

func (m *MainBar) Draw(region gui.Region) {
  m.region = region
  gl.Enable(gl.TEXTURE_2D)
  m.layout.Background.Data().Bind()
  gl.Color4d(1, 1, 1, 1)
  gl.Begin(gl.QUADS)
    gl.TexCoord2d(0, 0)
    gl.Vertex2i(region.X, region.Y)

    gl.TexCoord2d(0, -1)
    gl.Vertex2i(region.X, region.Y + region.Dy)

    gl.TexCoord2d(1,-1)
    gl.Vertex2i(region.X + region.Dx, region.Y + region.Dy)

    gl.TexCoord2d(1, 0)
    gl.Vertex2i(region.X + region.Dx, region.Y)
  gl.End()

  for _, button := range m.buttons {
    button.RenderAt(region.X, region.Y, m.mx, m.my)
  }

  if m.ent != nil {
    gl.Color4d(1, 1, 1, 1)
    m.ent.Still.Data().Bind()
    tdx := m.ent.Still.Data().Dx
    tdy := m.ent.Still.Data().Dy
    cx := region.X + m.layout.CenterStillFrame.X
    cy := region.Y + m.layout.CenterStillFrame.Y
    gl.Begin(gl.QUADS)
      gl.TexCoord2d(0, 0)
      gl.Vertex2i(cx - tdx / 2, cy - tdy / 2)

      gl.TexCoord2d(0, -1)
      gl.Vertex2i(cx - tdx / 2, cy + tdy / 2)

      gl.TexCoord2d(1,-1)
      gl.Vertex2i(cx + tdx / 2, cy + tdy / 2)

      gl.TexCoord2d(1, 0)
      gl.Vertex2i(cx + tdx / 2, cy - tdy / 2)
    gl.End()

    m.layout.Name.RenderString(m.ent.Name)
    m.layout.Ap.RenderString(fmt.Sprintf("Ap:%d", m.ent.Stats.ApCur()))
    m.layout.Hp.RenderString(fmt.Sprintf("Hp:%d", m.ent.Stats.HpCur()))
    m.layout.Corpus.RenderString(fmt.Sprintf("Corpus:%d", m.ent.Stats.Corpus()))
    m.layout.Ego.RenderString(fmt.Sprintf("Ego:%d", m.ent.Stats.Ego()))

    gl.Color4d(1, 1, 1, 1)
    m.layout.Divider.Data().Bind()
    tdx = m.layout.Divider.Data().Dx
    tdy = m.layout.Divider.Data().Dy
    cx = region.X + m.layout.Name.X
    cy = region.Y + m.layout.Name.Y - 5
    gl.Begin(gl.QUADS)
      gl.TexCoord2d(0, 0)
      gl.Vertex2i(cx - tdx / 2, cy - tdy / 2)

      gl.TexCoord2d(0, -1)
      gl.Vertex2i(cx - tdx / 2, cy + (tdy + 1) / 2)

      gl.TexCoord2d(1,-1)
      gl.Vertex2i(cx + (tdx + 1) / 2, cy + (tdy + 1) / 2)

      gl.TexCoord2d(1, 0)
      gl.Vertex2i(cx + (tdx + 1) / 2, cy - tdy / 2)
    gl.End()

    // Actions
    {
      spacing := m.layout.Actions.Icon_size * float64(m.layout.Actions.Count)
      spacing = m.layout.Actions.Width - spacing
      spacing /= float64(m.layout.Actions.Count - 1)
      s := m.layout.Actions.Icon_size
      num_actions := len(m.ent.Actions)
      xpos := m.layout.Actions.X

      if num_actions > m.layout.Actions.Count {
        xpos -= m.state.Actions.scroll_pos * (s + spacing)
      }
      d := base.GetDictionary(10)
      var r gui.Region
      r.X = int(m.layout.Actions.X)
      r.Y = int(m.layout.Actions.Y - d.MaxHeight())
      r.Dx = int(m.layout.Actions.Width)
      r.Dy = int(m.layout.Actions.Icon_size + d.MaxHeight())
      r.PushClipPlanes()

      gl.Color4d(1, 1, 1, 1)
      for i,action := range m.ent.Actions {

        // Highlight the selected action
        if action == m.ent.selected_action {
          gl.Disable(gl.TEXTURE_2D)
          gl.Color4d(1, 0, 0, 1)
          gl.Begin(gl.QUADS)
            gl.Vertex3d(xpos - 2, m.layout.Actions.Y - 2, 0)
            gl.Vertex3d(xpos - 2, m.layout.Actions.Y+s + 2, 0)
            gl.Vertex3d(xpos+s + 2, m.layout.Actions.Y+s + 2, 0)
            gl.Vertex3d(xpos+s + 2, m.layout.Actions.Y - 2, 0)
          gl.End()
        }
        gl.Enable(gl.TEXTURE_2D)
        action.Icon().Data().Bind()
        if action.Preppable(m.ent, m.game) {
          gl.Color4d(1, 1, 1, 1)
        } else {
          gl.Color4d(0.5, 0.5, 0.5, 1)
        }
        gl.Begin(gl.QUADS)
          gl.TexCoord2d(0, 0)
          gl.Vertex3d(xpos, m.layout.Actions.Y, 0)

          gl.TexCoord2d(0, -1)
          gl.Vertex3d(xpos, m.layout.Actions.Y+s, 0)

          gl.TexCoord2d(1, -1)
          gl.Vertex3d(xpos+s, m.layout.Actions.Y+s, 0)

          gl.TexCoord2d(1, 0)
          gl.Vertex3d(xpos+s, m.layout.Actions.Y, 0)
        gl.End()
        gl.Disable(gl.TEXTURE_2D)

        ypos := m.layout.Actions.Y - d.MaxHeight() - 2
        d.RenderString(fmt.Sprintf("%d", i+1), xpos + s / 2, ypos, 0, d.MaxHeight(), gui.Center)

        xpos += spacing + m.layout.Actions.Icon_size
      }

      r.PopClipPlanes()

      // Now, if there is a selected action, position it between the arrows
      if m.state.Actions.selected != nil {
        // a := m.state.Actions.selected
        d := base.GetDictionary(15)
        x := m.layout.Actions.X + m.layout.Actions.Width / 2
        y := float64(m.layout.ActionLeft.Y)
        str := fmt.Sprintf("%s:%dAP", m.state.Actions.selected.String(), m.state.Actions.selected.AP())
        gl.Color4d(1, 1, 1, 1)
        d.RenderString(str, x, y, 0, d.MaxHeight(), gui.Center)
      }
    }
  }

  {
    gl.Color4d(1, 1, 1, 1)
    c := m.layout.Conditions
    d := base.GetDictionary(int(c.Size))
    ypos := c.Height + c.Y
    var r gui.Region
    r.X = int(c.X)
    r.Y = int(c.Y)
    r.Dx = int(c.Width)
    r.Dy = int(c.Height)
    r.PushClipPlanes()

    for _,s := range []string{"Blinded!", "Terrified", "Vexed!", "Offended!", "Hypnotized!", "Conned!", "Scorhed!"} {
      d.RenderString(s, c.X + c.Width / 2, ypos - float64(d.MaxHeight()), 0, d.MaxHeight(), gui.Center)
      ypos -= float64(d.MaxHeight())
      ypos += 3
    }

    r.PopClipPlanes()
  }
}

func (m *MainBar) DrawFocused(region gui.Region) {

}

func (m *MainBar) String() string {
  return "main bar"
}

