# Gliff: a lightweight TUI framework

Gliff provides a simple framework for building TUI applications. It has

- A simple design
- An efficient, optimizing renderer
- Mouse support, including built-in hit-testing

## Building Gliff apps

Applications in Gliff are built out of models, where each model implements the
`Component` interface:

```go
type Component interface {
	Init() Cmd
	Update(Msg) Cmd
	Render() string
}
```

Components process incoming messages, update their own state accordingly, and
can return commands to trigger subsequent actions.

Commands are functions that return messages. Each command is run in its own
goroutine, and whatever message it returns will be fed back into the main event
loop. This makes is easy to perform some work asynchronously and then report
back with some results.

Messages can be any type you like. Built-in messages include:

- `KeyMsg` representing incoming keypresses
- `MouseMsg` representing mouse clicks
- `QuitMsg` tells the application to exit gracefully

Components can contain other components, to form a hierarchy. To run the
top-level component, you use a `tui.Application`:

```go
m := NewAppModel()
app := tui.NewApplication(m)
err := app.Run()
```

You can see some simple examples of Gliff applications in the `./examples` folder.

## Handling mouse clicks

Mouse clicks will be reported as instances of `MouseMsg`. A `MouseMsg` contains
the X & Y coordinates of the character that was clicked on.

But often what you'd actually want to know is whether a particular piece of
content was clicked on. To avoid having to calculate click positions by hand,
you can instead mark parts of the content as named "targets". Whenever a click
occurs on a target (no matter where on the screen it ended up), the `MouseMsg`
will have the target name populated in its `Target` field.

You can designate some content as a target with `WithTarget`:

```go
content := "Hello! " + tui.WithTarget("button", "Click here!")
```

A click on the "Click here!" portion of that string would trigger a `MouseMsg`
with its `Target` set to `button` (while a click on the "Hello!" part would
not).
