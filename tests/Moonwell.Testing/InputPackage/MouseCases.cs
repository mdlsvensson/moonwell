using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>Cases for each player's mouse: the position it keeps, the buttons, the reactions, and the local reads and changes, on the fake game.</summary>
    public static class MouseCases
    {
        /// <summary>Makes slots 0 and 1 user players, installs the package on a fake world with the mouse on, and returns it.</summary>
        private static FakeInputWorld Fresh(FakeWorld world, bool mouse = true)
        {
            world.SetSlot(0, SlotState.Playing, Controller.User);
            world.SetSlot(1, SlotState.Playing, Controller.User);
            var fake = new FakeInputWorld();
            InputSystem.Install(fake, mouse, new string[0], new float[0]);
            return fake;
        }

        /// <summary>Fails unless the log is exactly these lines.</summary>
        private static void ExpectLog(List<string> log, string[] expected, string what)
        {
            Expect.Equal(string.Join(", ", expected), string.Join(", ", log.ToArray()), what);
        }

        /// <summary>A move event sets X, Y and Position, and HasPosition.</summary>
        public static void Moved(FakeWorld world)
        {
            var fake = Fresh(world);
            var mouse = Player.One.Mouse();
            Expect.True(!mouse.HasPosition, "no position before an event");
            fake.Move(Player.One, 100f, 200f);
            Expect.Near(100f, mouse.X, "X");
            Expect.Near(200f, mouse.Y, "Y");
            Expect.Near(100f, mouse.Position.X, "Position.X");
            Expect.Near(200f, mouse.Position.Y, "Position.Y");
            Expect.True(mouse.HasPosition, "HasPosition");
            Expect.True(!Player.Two.Mouse().HasPosition, "another player's mouse is not moved");
        }

        /// <summary>A reading of (0, 0) or of the world's top-right corner less 32 is not a place: the position is kept. Both coordinates must match, and the corner less 31 is a place.</summary>
        public static void NotAPlace(FakeWorld world)
        {
            var fake = Fresh(world);
            var mouse = Player.One.Mouse();
            fake.Move(Player.One, 100f, 200f);
            fake.Move(Player.One, 0f, 0f);
            Expect.Near(100f, mouse.X, "X kept on (0, 0)");
            Expect.Near(200f, mouse.Y, "Y kept on (0, 0)");
            fake.Move(Player.One, 4064f, 4064f);
            Expect.Near(100f, mouse.X, "X kept on the corner less 32");
            Expect.Near(200f, mouse.Y, "Y kept on the corner less 32");
            fake.Move(Player.One, 4064f, 150f);
            Expect.Near(4064f, mouse.X, "one coordinate on the corner is a place: X");
            Expect.Near(150f, mouse.Y, "one coordinate on the corner is a place: Y");
            fake.Move(Player.One, 0f, 90f);
            Expect.Near(90f, mouse.Y, "one coordinate at 0 is a place");
            fake.Move(Player.One, 4065f, 4065f);
            Expect.Near(4065f, mouse.X, "the corner less 31 moves it: X");
            Expect.Near(4065f, mouse.Y, "the corner less 31 moves it: Y");
            Expect.Equal(1, fake.WorldReads, "the world's corner is read once");
        }

        /// <summary>The corner follows the world's size, read at Install.</summary>
        public static void CornerOfTheWorld(FakeWorld world)
        {
            world.SetSlot(0, SlotState.Playing, Controller.User);
            var fake = new FakeInputWorld();
            fake.MaxX = 2048f;
            fake.MaxY = 1024f;
            InputSystem.Install(fake, true, new string[0], new float[0]);
            var mouse = Player.One.Mouse();
            fake.Move(Player.One, 100f, 200f);
            fake.Move(Player.One, 2016f, 992f);
            Expect.Near(100f, mouse.X, "X kept on this world's corner less 32");
            fake.Move(Player.One, 4064f, 4064f);
            Expect.Near(4064f, mouse.X, "the other world's corner is a place here");
        }

        /// <summary>When the first event reads (0, 0) there is no position yet.</summary>
        public static void FirstNotAPlace(FakeWorld world)
        {
            var fake = Fresh(world);
            var mouse = Player.One.Mouse();
            fake.Move(Player.One, 0f, 0f);
            Expect.True(!mouse.HasPosition, "still no position");
            Expect.Near(0f, mouse.Position.X, "Position.X is zero");
            Expect.Near(0f, mouse.Position.Y, "Position.Y is zero");
            fake.Press(Player.One, MouseButton.Left, 4064f, 4064f);
            Expect.True(!mouse.HasPosition, "the corner on a press is not a place either");
        }

        /// <summary>A press and a release set IsDown, and set the position too.</summary>
        public static void Buttons(FakeWorld world)
        {
            var fake = Fresh(world);
            var mouse = Player.One.Mouse();
            fake.Press(Player.One, MouseButton.Right, 300f, 400f);
            Expect.True(mouse.IsDown(MouseButton.Right), "right is down");
            Expect.True(!mouse.IsDown(MouseButton.Left), "left is not");
            Expect.True(!mouse.IsDown(MouseButton.Middle), "middle is not");
            Expect.Near(300f, mouse.X, "the press set X");
            Expect.Near(400f, mouse.Y, "the press set Y");
            fake.Press(Player.One, MouseButton.Left, 310f, 410f);
            Expect.True(mouse.IsDown(MouseButton.Left) && mouse.IsDown(MouseButton.Right), "both are down");
            fake.Release(Player.One, MouseButton.Right, 320f, 420f);
            Expect.True(!mouse.IsDown(MouseButton.Right), "right is up");
            Expect.True(mouse.IsDown(MouseButton.Left), "left is still down");
            Expect.Near(320f, mouse.X, "the release set X");
            Expect.Near(420f, mouse.Y, "the release set Y");
            fake.Release(Player.One, MouseButton.Left, 320f, 420f);
            Expect.True(!mouse.IsDown(MouseButton.Left), "left is up");
            fake.Press(Player.One, MouseButton.Middle, 1f, 2f);
            Expect.True(mouse.IsDown(MouseButton.Middle), "middle is down");
            Expect.True(!Player.Two.Mouse().IsDown(MouseButton.Middle), "another player's mouse is not");
        }

        /// <summary>An event of Player 1 runs the game's components in add order, then Player 1's, never Player 2's; the reactions give the mouse and the button and run with the component as the current owner.</summary>
        public static void Order(FakeWorld world)
        {
            var fake = Fresh(world);
            var log = new List<string>();
            var first = new LogMouseGame(log, "game a");
            var one = new LogMousePlayer(log, "player 1");
            var two = new LogMousePlayer(log, "player 2");
            var second = new LogMouseGame(log, "game b");
            Game.Instance.Add(first);
            Player.One.Add(one);
            Player.Two.Add(two);
            Game.Instance.Add(second);
            fake.Move(Player.One, 10f, 20f);
            ExpectLog(log, new[] { "game a moved", "game b moved", "player 1 moved" }, "a move of Player 1");
            Expect.Same(first, first.OwnerSeen, "the game's component is the owner");
            Expect.Same(one, one.OwnerSeen, "the player's component is the owner");
            Expect.True(two.OwnerSeen == null, "Player 2's component was not called");
            log.Clear();
            fake.Press(Player.Two, MouseButton.Right, 10f, 20f);
            ExpectLog(log, new[] { "game a pressed Right", "game b pressed Right", "player 2 pressed Right" }, "a press of Player 2");
            log.Clear();
            fake.Release(Player.Two, MouseButton.Right, 10f, 20f);
            ExpectLog(log, new[] { "game a released Right", "game b released Right", "player 2 released Right" }, "a release of Player 2");
        }

        /// <summary>A component that writes only OnMousePressed is called on a press and on nothing else.</summary>
        public static void OnlyWritten(FakeWorld world)
        {
            var fake = Fresh(world);
            var log = new List<string>();
            Game.Instance.Add(new PressOnlyMouse(log));
            fake.Move(Player.One, 10f, 20f);
            Expect.Equal(0, log.Count, "not called on a move");
            fake.Press(Player.One, MouseButton.Left, 10f, 20f);
            ExpectLog(log, new[] { "press only pressed Left" }, "called on a press");
            fake.Release(Player.One, MouseButton.Left, 10f, 20f);
            Expect.Equal(1, log.Count, "not called on a release");
        }

        /// <summary>A reaction that throws is reported as Class.Method and the next component still runs, with the mouse already up to date.</summary>
        public static void Throwing(FakeWorld world)
        {
            var fake = Fresh(world);
            var log = new List<string>();
            Game.Instance.Add(new ThrowingMouse());
            Game.Instance.Add(new LogMouseGame(log, "after"));
            fake.Move(Player.One, 10f, 20f);
            Expect.Equal(1, world.Logged.Count, "one line logged");
            Expect.Equal("Moonwell: ThrowingMouse.OnMouseMoved failed: boom", world.Logged[0], "the logged line");
            ExpectLog(log, new[] { "after moved" }, "the next component ran");
            Expect.Near(10f, Player.One.Mouse().X, "the position was set");
        }

        /// <summary>The package listens only for user players playing at that moment, in slot order: not a computer, not a user who left; their mice exist but never move.</summary>
        public static void ComputerAndLeft(FakeWorld world)
        {
            world.SetSlot(0, SlotState.Playing, Controller.User);
            world.SetSlot(1, SlotState.Playing, Controller.Computer);
            world.SetSlot(2, SlotState.Left, Controller.User);
            world.SetSlot(3, SlotState.Playing, Controller.User);
            var fake = new FakeInputWorld();
            InputSystem.Install(fake, true, new string[0], new float[0]);
            Expect.Equal(2, fake.Listens.Count, "two ListenMouse calls");
            Expect.Same(Player.One.Handle, fake.Listens[0].Player, "the first user");
            Expect.Same(Player.Four.Handle, fake.Listens[1].Player, "the second user");
            for (var slot = 1; slot < 3; slot++)
            {
                var mouse = Player.Get(slot).Mouse();
                Expect.True(!mouse.HasPosition, "no position in slot " + slot);
                Expect.True(!mouse.IsDown(MouseButton.Left) && !mouse.IsDown(MouseButton.Middle) && !mouse.IsDown(MouseButton.Right), "nothing down in slot " + slot);
            }
        }

        /// <summary>Install with the mouse off listens for nothing, and the mice still exist.</summary>
        public static void NoMouse(FakeWorld world)
        {
            var fake = Fresh(world, false);
            Expect.Equal(0, fake.Listens.Count, "no ListenMouse call");
            Expect.True(!Player.One.Mouse().HasPosition, "the mouse exists and has no position");
        }

        /// <summary>One mouse per slot from 0 to 27, each of its own player; Install again starts them anew; a read before Install throws.</summary>
        public static void PerSlot(FakeWorld world)
        {
            var fake = Fresh(world);
            for (var slot = 0; slot < 28; slot++)
            {
                var mouse = Player.Get(slot).Mouse();
                Expect.Same(Player.Get(slot), mouse.Player, "the player of the mouse in slot " + slot);
                Expect.Same(mouse, Player.Get(slot).Mouse(), "the same mouse again in slot " + slot);
            }

            fake.Move(Player.One, 5f, 6f);
            InputSystem.Install(new FakeInputWorld(), false, new string[0], new float[0]);
            Expect.True(!Player.One.Mouse().HasPosition, "a new install starts the mice anew");
            InputSystem.Uninstall();
            Expect.Throws<InvalidOperationException>(() => Player.One.Mouse(), "a read before Install");
        }

        /// <summary>The machine-local reads give what this machine's game gives, inside Local.For.</summary>
        public static void Local(FakeWorld world)
        {
            var fake = Fresh(world);
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            fake.ScreenX = 640;
            fake.ScreenY = 480;
            fake.Focus = unit.Handle;
            fake.Held[(int)MouseButton.Left] = true;
            var mouse = Player.One.Mouse();
            var x = 0;
            var y = 0;
            Unit? focus = null;
            var left = false;
            var right = true;
            Moonwell.Local.For(Player.One, () =>
            {
                x = mouse.ScreenX;
                y = mouse.ScreenY;
                focus = mouse.FocusUnit;
                left = mouse.IsHeldNow(MouseButton.Left);
                right = mouse.IsHeldNow(MouseButton.Right);
            });
            Expect.Equal(640, x, "ScreenX");
            Expect.Equal(480, y, "ScreenY");
            Expect.Same(unit, focus, "FocusUnit");
            Expect.True(left, "left held now");
            Expect.True(!right, "right not held now");
            fake.Focus = null;
            Moonwell.Local.For(Player.One, () => focus = mouse.FocusUnit);
            Expect.True(focus == null, "no unit under the cursor");
        }

        /// <summary>SetScreenPosition and ShowCursor reach the world with the mouse's player.</summary>
        public static void Setters(FakeWorld world)
        {
            var fake = Fresh(world);
            Player.One.Mouse().SetScreenPosition(10, 20);
            Player.Two.Mouse().ShowCursor(false);
            Player.One.Mouse().ShowCursor(true);
            ExpectLog(fake.Calls, new[] { "screen 0 10 20", "cursor 1 hide", "cursor 0 show" }, "the changes");
        }
    }
}
