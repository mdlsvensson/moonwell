using System;
using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell.Testing
{
    /// <summary>Cases for the full audit's fixes to input: which mouse events the package listens to, the watchers it runs, and World Editor's cameras named before it starts.</summary>
    public static class AuditInputCases
    {
        /// <summary>Makes slots 0 and 1 user players.</summary>
        private static void TwoUsers(FakeWorld world)
        {
            world.SetSlot(0, SlotState.Playing, Controller.User);
            world.SetSlot(1, SlotState.Playing, Controller.User);
        }

        /// <summary>With the mouse on and its moves off (a map whose code reads no synced position and writes no OnMouseMoved), the package listens to each user's presses and releases and not to their moves; a press still gives the position and the button.</summary>
        public static void NoMoves(FakeWorld world)
        {
            TwoUsers(world);
            var fake = new FakeInputWorld();
            InputSystem.Install(fake, true, false, new string[0], new float[0]);
            Expect.Equal(2, fake.Listens.Count, "both users are listened to");
            for (var i = 0; i < fake.Listens.Count; i++)
            {
                Expect.True(fake.Listens[i].Moved == null, "no move event for user " + i);
                Expect.True(fake.Listens[i].Pressed != null && fake.Listens[i].Released != null, "presses and releases for user " + i);
            }

            var mouse = Player.One.Mouse();
            fake.Press(Player.One, MouseButton.Left, 300f, 400f);
            Expect.True(mouse.IsDown(MouseButton.Left), "the press is heard");
            Expect.Near(300f, mouse.X, "and gives the position");
            fake.Release(Player.One, MouseButton.Left, 310f, 410f);
            Expect.True(!mouse.IsDown(MouseButton.Left), "the release is heard");
        }

        /// <summary>Fails unless the log is exactly these lines.</summary>
        private static void ExpectLog(List<string> log, string[] expected, string what)
        {
            Expect.Equal(string.Join(", ", expected), string.Join(", ", log.ToArray()), what);
        }

        /// <summary>
        /// A mouse event runs the watchers without looking at every game component again: with 50 game components that do
        /// not watch the mouse (areas, say) and one that does, 100 moves look at the components once. A watcher added later
        /// is called from the next event, and one that ended is not called.
        /// </summary>
        public static void WatchersKept(FakeWorld world)
        {
            TwoUsers(world);
            var fake = new FakeInputWorld();
            InputSystem.Install(fake, true, true, new string[0], new float[0]);
            for (var i = 0; i < 50; i++)
            {
                Game.Instance.Add(new QuietGame());
            }

            var log = new List<string>();
            var first = new LogMouseGame(log, "first");
            Game.Instance.Add(first);
            MouseReactions.Lookups = 0;
            for (var i = 0; i < 100; i++)
            {
                fake.Move(Player.One, 10f + i, 20f);
            }

            Expect.Equal(100, log.Count, "the watcher is called at each move");
            Expect.True(MouseReactions.Lookups <= 51, "the components are looked at once, not at each move: " + MouseReactions.Lookups);
            log.Clear();
            var second = new LogMouseGame(log, "second");
            Game.Instance.Add(second);
            fake.Move(Player.One, 1f, 2f);
            ExpectLog(log, new[] { "first moved", "second moved" }, "a watcher added is called from the next event");
            log.Clear();
            first.End();
            fake.Press(Player.One, MouseButton.Left, 1f, 2f);
            ExpectLog(log, new[] { "second pressed Left" }, "one that ended is not");
            log.Clear();
            var quiet = Game.Instance.Get<QuietGame>()!;
            quiet.End();
            var third = new LogMouseGame(log, "third");
            Game.Instance.Add(third);
            fake.Move(Player.One, 1f, 2f);
            ExpectLog(log, new[] { "second moved", "third moved" }, "one ended and one added, the count the same: the new one is called");
        }

        /// <summary>A watcher added by a watcher's reaction is called from the next event, and one ended by an earlier watcher's reaction in the same event is not called; a player's watchers are kept apart from another player's.</summary>
        public static void WatchersChangedInDispatch(FakeWorld world)
        {
            TwoUsers(world);
            var fake = new FakeInputWorld();
            InputSystem.Install(fake, true, true, new string[0], new float[0]);
            var log = new List<string>();
            var late = new LogMouseGame(log, "late");
            var adder = new AddingMouse(log, new LogMouseGame(log, "added"), late);
            Game.Instance.Add(adder);
            Game.Instance.Add(late);
            fake.Press(Player.One, MouseButton.Left, 1f, 2f);
            ExpectLog(log, new[] { "adder pressed" }, "the added one waits for the next event, and the ended one is not called");
            log.Clear();
            fake.Press(Player.One, MouseButton.Left, 1f, 2f);
            ExpectLog(log, new[] { "adder pressed", "added pressed Left" }, "the added one at the next event");
            log.Clear();
            Player.One.Add(new LogMousePlayer(log, "one"));
            Player.Two.Add(new LogMousePlayer(log, "two"));
            fake.Release(Player.Two, MouseButton.Left, 1f, 2f);
            fake.Release(Player.One, MouseButton.Left, 1f, 2f);
            ExpectLog(log, new[] { "added released Left", "two released Left", "added released Left", "one released Left" }, "each player's own watchers");
        }

        /// <summary>
        /// A component with a field set to <c>CameraSetup.Named("Intro")</c>, made before the package starts (a component listed in
        /// the map's <c>Components</c> is made in its constructor, before the installs), is made without an error, and the setup
        /// is looked up when applied; a name the map does not have throws ArgumentException there. After the start, Named gives
        /// the installed setup, the same each time.
        /// </summary>
        public static void CameraNamedBeforeStart(FakeWorld world)
        {
            TwoUsers(world);
            InputSystem.Uninstall();
            var tour = new IntroTour();
            var unknown = CameraSetup.Named("Nowhere");
            Expect.Equal("Intro", tour.Intro.Name, "the setup has its name");
            var fake = new FakeInputWorld();
            InputSystem.Install(fake, false, false, new[] { "Intro" }, new[] { 100f, 200f, 0f, -1f });
            Game.Instance.Add(tour);
            tour.Show(Player.Two);
            Expect.Equal(1, fake.CameraCalls.Count, "one apply");
            Expect.Equal("apply", fake.CameraCalls[0].Kind, "an apply");
            Expect.Same(fake.Made[0].Setup, fake.CameraCalls[0].Setup, "of World Editor's Intro");
            Expect.Throws<ArgumentException>(() => Player.One.Camera().Apply(unknown), "an unknown name throws when applied");
            Expect.Same(CameraSetup.Named("Intro"), CameraSetup.Named("Intro"), "after the start, the same setup each time");
            Expect.Throws<ArgumentException>(() => CameraSetup.Named("Nowhere"), "and an unknown name throws at once");
        }

        /// <summary>With the mouse and its moves on, all three events are listened to; with the mouse off, none, whatever the moves say.</summary>
        public static void Moves(FakeWorld world)
        {
            TwoUsers(world);
            var fake = new FakeInputWorld();
            InputSystem.Install(fake, true, true, new string[0], new float[0]);
            Expect.True(fake.Listens.Count == 2 && fake.Listens[0].Moved != null && fake.Listens[1].Moved != null, "moves for both users");
            fake.Move(Player.Two, 50f, 60f);
            Expect.Near(50f, Player.Two.Mouse().X, "a move is heard");
            var off = new FakeInputWorld();
            InputSystem.Install(off, false, true, new string[0], new float[0]);
            Expect.Equal(0, off.Listens.Count, "no mouse, no events");
        }
    }

    /// <summary>A game component that does not watch the mouse.</summary>
    public sealed class QuietGame : GameComponent
    {
    }

    /// <summary>A game component with World Editor's "Intro" camera in a field, as a map writes it.</summary>
    public sealed class IntroTour : GameComponent
    {
        /// <summary>The camera, named when the component is made.</summary>
        public readonly CameraSetup Intro = CameraSetup.Named("Intro");

        /// <summary>Applies the camera to a player.</summary>
        public void Show(Player player)
        {
            player.Camera().Apply(Intro, 1f);
        }
    }

    /// <summary>A game component whose press reaction logs "adder pressed", adds a watcher to the game the first time, and ends another.</summary>
    public sealed class AddingMouse : GameComponent, IMouseWatch
    {
        private readonly List<string> log;

        private readonly GameComponent toAdd;

        private readonly GameComponent toEnd;

        private bool done;

        /// <summary>Makes it with the log, the watcher it adds and the one it ends.</summary>
        public AddingMouse(List<string> log, GameComponent added, GameComponent ended)
        {
            this.log = log;
            toAdd = added;
            toEnd = ended;
        }

        /// <summary>Logs, and the first time adds one watcher and ends the other.</summary>
        public void OnMousePressed(Mouse mouse, MouseButton button)
        {
            log.Add("adder pressed");
            if (!done)
            {
                done = true;
                Game.Instance.Add(toAdd);
                toEnd.End();
            }
        }
    }
}
