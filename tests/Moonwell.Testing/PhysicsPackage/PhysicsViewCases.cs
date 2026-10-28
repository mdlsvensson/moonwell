using Moonwell.Engine;

namespace Moonwell.Testing
{
#if DEBUG
    /// <summary>Cases for the physics package's view in the debug mode: a selected missile's and pushed unit's lines, and the statistics' line.</summary>
    public static class PhysicsViewCases
    {
        /// <summary>The type id the Pushee class is registered with.</summary>
        private const string PusheeId = "h0P1";

        /// <summary>Installs the interactions, physics and terrain packages on fakes (the terrain flat at 0 over 16 by 16 tiles from (0, 0)), registers Pushee, and turns the debug mode on with -debug.</summary>
        private static FakeDebugWorld On(FakeWorld world)
        {
            Interactions.Install(new FakeInteractionWorld());
            Physics.Install(new FakePhysicsWorld());
            var corner = GridText.Corner(0.0f, 2);
            var row = "";
            for (var i = 0; i < 17; i++)
            {
                row = row + corner;
            }

            var text = "";
            for (var i = 0; i < 17; i++)
            {
                text = text + row;
            }

            Terrain.Install(new FakeTerrainWorld(), 17, 17, 0.0f, 0.0f, text);
            ObjectTypes.Unit(ObjectTypes.FourCC(PusheeId), () => new Pushee());
            TestLog.Lines.Clear();
            var debug = FakeDebugWorld.Install(world);
            debug.Chat(0, "-debug");
            return debug;
        }

        /// <summary>Makes a Pushee at a place, and takes it in.</summary>
        private static Pushee PusheeAt(FakeWorld world, float x, float y)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC(PusheeId));
            world.MoveUnit(handle, x, y);
            return (Pushee)Unit.Of(handle)!;
        }

        /// <summary>The panel's line that starts with a text; null when none does.</summary>
        private static string? LineStarting(FakeDebugWorld debug, string start)
        {
            var lines = debug.PanelLines();
            for (var i = 0; i < lines.Count; i++)
            {
                if (lines[i].StartsWith(start))
                {
                    return lines[i];
                }
            }

            return null;
        }

        /// <summary>The view is added once with the package; a selected missile's line gives its speed, velocity and radius; a pushed unit's gives its push, and a unit not pushed has none; a lifted one's says how far up it is.</summary>
        public static void Lines(FakeWorld world)
        {
            var debug = On(world);
            Expect.True(DebugViews.All.Contains(PhysicsView.Instance), "the view is added with the package");
            var dart = new Dart();
            dart.Velocity = new Vector3(300.0f, 400.0f, 0.0f);
            dart.Radius = 24.0f;
            Missile.Create(dart, 512.0f, 512.0f, 100.0f);
            DebugMode.Select(dart);
            Expect.Equal("speed 500 · velocity 300, 400, 0 · radius 24", LineStarting(debug, "speed "), "the missile's line");
            Expect.Same(null, LineStarting(debug, "pushed"), "no push line for a missile");

            var unit = PusheeAt(world, 1024.0f, 1024.0f);
            DebugMode.Select(unit);
            Expect.Same(null, LineStarting(debug, "pushed"), "no push line for a unit not pushed");
            Expect.Same(null, LineStarting(debug, "speed "), "no missile line for a unit");
            unit.Push(400.0f, -100.0f);
            DebugMode.Select(dart);
            DebugMode.Select(unit);
            Expect.Equal("pushed: velocity 400, -100, 0", LineStarting(debug, "pushed"), "the pushed unit's line");

            var lifted = PusheeAt(world, 1536.0f, 512.0f);
            lifted.Push(0.0f, 0.0f, 600.0f);
            Steps(world, 8);
            DebugMode.Select(lifted);
            Expect.Equal("pushed: velocity 0, 0, 400 · lifted 125 up", LineStarting(debug, "pushed"), "a lifted unit's line: rising at 400, 125 up a quarter second after it was pushed up at 600");
        }

        /// <summary>Moves the game on by a number of steps, one step at a time.</summary>
        private static void Steps(FakeWorld world, int count)
        {
            for (var i = 0; i < count; i++)
            {
                world.Advance(Interactions.Step);
            }
        }

        /// <summary>-stats gives the missiles that have not ended, how many of them rest, and the units being pushed; a ball that comes to rest counts as resting.</summary>
        public static void Stats(FakeWorld world)
        {
            var debug = On(world);
            var first = new Dart();
            first.Speed = 300.0f;
            Missile.Create(first, 512.0f, 512.0f, 100.0f);
            var second = new Dart();
            second.Speed = 300.0f;
            Missile.Create(second, 256.0f, 256.0f, 100.0f);
            var ended = new Dart();
            Missile.Create(ended, 768.0f, 768.0f, 100.0f);
            ended.End();
            PusheeAt(world, 1024.0f, 1024.0f).Push(400.0f, 0.0f);
            debug.Chat(0, "-stats");
            Expect.Equal("physics: 2 missiles (0 resting) · 1 pushed unit", LineStarting(debug, "physics: "), "the statistics' line");

            var ball = new Ball();
            ball.GroundContact = Contact.Slide;
            ball.Gravity = 800.0f;
            ball.Friction = 1000.0f;
            Missile.Create(ball, 1536.0f, 1536.0f, 16.0f);
            for (var i = 0; i < 32 && ball.Rested == 0; i++)
            {
                Steps(world, 1);
            }

            Expect.Equal(1, ball.Rested, "the ball rests");
            // -stats again turns them off, and once more on, with the counts as they are now.
            debug.Chat(0, "-stats");
            debug.Chat(0, "-stats");
            Expect.Equal("physics: 3 missiles (1 resting) · 1 pushed unit", LineStarting(debug, "physics: "), "with a ball resting");
        }
    }
#endif
}
