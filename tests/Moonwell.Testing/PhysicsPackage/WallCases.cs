using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>Cases for walls: missiles bouncing off, sliding along, flying over and caught fast at a wall, and a pushed unit stopped by one; on flat ground 2,048 across with the default step of 1/32 second.</summary>
    public static class WallCases
    {
        /// <summary>The type id the Pushee class is registered with.</summary>
        private const string PusheeId = "h0P1";

        /// <summary>Installs the interactions, physics and terrain packages on fakes, the terrain flat at 0 over 16 by 16 tiles from (0, 0), and registers Pushee.</summary>
        private static void Fresh()
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
        }

        /// <summary>Places a shot with a WallContact at a place with a velocity, and gives it back.</summary>
        private static Shot ShotAt(Contact contact, float x, float y, float z, float vx, float vy)
        {
            var shot = new Shot();
            shot.WallContact = contact;
            Missile.Create(shot, x, y, z);
            shot.VX = vx;
            shot.VY = vy;
            return shot;
        }

        /// <summary>Moves the game on by a number of steps, one step at a time.</summary>
        private static void Steps(FakeWorld world, int count)
        {
            for (var i = 0; i < count; i++)
            {
                world.Advance(Interactions.Step);
            }
        }

        /// <summary>Moves the game on a step at a time, at most a number of steps, until a shot has had a number of impacts.</summary>
        private static void UntilImpacts(FakeWorld world, Shot shot, int impacts, int most)
        {
            for (var i = 0; i < most && shot.Impacts.Count < impacts; i++)
            {
                world.Advance(Interactions.Step);
            }
        }

        /// <summary>Fails unless two floats are within a tolerance.</summary>
        private static void Within(float expected, float actual, float tolerance, string what)
        {
            Expect.True(Math.Abs(expected - actual) <= tolerance, what + ": expected " + expected + " within " + tolerance + ", was " + actual);
        }

        /// <summary>What an impact read inside OnImpact: index 0 kind, 1 to 3 the point, 4 to 6 the normal, 7 the speed.</summary>
        private static float Read(Shot shot, int impact, int index)
        {
            return shot.Reads[impact * 8 + index];
        }

        /// <summary>
        /// A ball at 45 degrees into an endless wall along y with WallContact Bounce: it meets it once, at the wall, with the
        /// normal out of the side it came from and the speed into it; the speed into it is reversed and halved (its
        /// Bounciness 0.5 times the wall's 1) and the speed along it kept. One whose CanHit refuses the wall flies through it,
        /// asked once. A wall of Bounciness 0.5 keeps a quarter. The wall
        /// belongs to the owner current at Create, or the one given, and ends with it; a ball then flies through where it was.
        /// </summary>
        public static void Bounce(FakeWorld world)
        {
            Fresh();
            var maker = Owner.Current;
            var wall = Wall.Create(600.0f, 200.0f, 600.0f, 800.0f);
            Expect.Same(maker, wall.Owner, "it belongs to its maker");
            Within(600.0f, wall.X1, 0.0f, "its ends: x1");
            Within(800.0f, wall.Y2, 0.0f, "y2");
            Within(0.0f, wall.Height, 0.0f, "endless");
            var ball = ShotAt(Contact.Bounce, 400.0f, 500.0f, 100.0f, 600.0f, 600.0f);
            UntilImpacts(world, ball, 1, 64);
            Expect.Equal(1, ball.Impacts.Count, "it met the wall");
            Expect.Same(wall, ball.Met[0], "the wall is the impact's Other");
            Within((float)(int)ImpactKind.Wall, Read(ball, 0, 0), 0.0f, "of a wall");
            Within(600.0f, Read(ball, 0, 1), 0.01f, "at the wall: x");
            Within(684.0f, Read(ball, 0, 2), 0.01f, "y");
            Within(100.0f, Read(ball, 0, 3), 0.01f, "z");
            Within(-1.0f, Read(ball, 0, 4), 0.0001f, "the normal out of the side it came from: x");
            Within(0.0f, Read(ball, 0, 5), 0.0001f, "y");
            Within(0.0f, Read(ball, 0, 6), 0.0001f, "level");
            Within(600.0f, Read(ball, 0, 7), 0.01f, "the speed into it");
            Within(-300.0f, ball.VX, 0.01f, "the speed into it reversed and halved");
            Within(600.0f, ball.VY, 0.01f, "the speed along it kept");
            Within(584.0f, ball.X, 0.01f, "put where it touched");
            Steps(world, 16);
            Expect.Equal(1, ball.Impacts.Count, "it flies off and does not meet it again");

            var ghost = new Shot();
            ghost.WallContact = Contact.Stop;
            ghost.Hit = other => !(other is Wall);
            Missile.Create(ghost, 400.0f, 300.0f, 100.0f);
            ghost.VX = 600.0f;
            Steps(world, 24);
            Expect.Equal(0, ghost.Impacts.Count, "one whose CanHit refuses the wall flies through it");
            Expect.True(ghost.Asked.Count == 1 && ghost.Asked[0] == wall, "asked once, though inside it for steps");

            var log = new List<string>();
            var keeper = new TestOwner("keeper", log);
            var soft = Wall.Create(600.0f, 1200.0f, 600.0f, 1800.0f, 0.0f, 0.5f, keeper);
            Expect.Same(keeper, soft.Owner, "a wall given an owner belongs to it");
            Within(0.5f, soft.Bounciness, 0.0f, "its Bounciness");
            var dull = ShotAt(Contact.Bounce, 400.0f, 1500.0f, 100.0f, 600.0f, 0.0f);
            UntilImpacts(world, dull, 1, 64);
            Expect.Equal(1, dull.Impacts.Count, "it met the soft wall");
            Within(-150.0f, dull.VX, 0.01f, "the two bouncinesses multiplied: a quarter kept");

            keeper.End();
            Expect.True(soft.HasEnded, "the wall ended with its owner");
            var through = ShotAt(Contact.Bounce, 400.0f, 1500.0f, 100.0f, 600.0f, 0.0f);
            Steps(world, 24);
            Expect.Equal(0, through.Impacts.Count, "nothing is met where it was");
            Expect.True(through.X > 700.0f, "it flew on; at " + through.X);
        }

        /// <summary>
        /// WallContact Slide: a ball at 45 degrees, 600 into the wall, bounces off it as Bounce does; one meeting it at 4 a
        /// second, slower than 5, loses the speed into it and slides along it, keeping the rest, with no more impacts.
        /// </summary>
        public static void Slide(FakeWorld world)
        {
            Fresh();
            Wall.Create(600.0f, 200.0f, 600.0f, 1800.0f);
            var fast = ShotAt(Contact.Slide, 400.0f, 300.0f, 100.0f, 600.0f, 600.0f);
            UntilImpacts(world, fast, 1, 64);
            Expect.Equal(1, fast.Impacts.Count, "the fast ball met the wall");
            Within(-300.0f, fast.VX, 0.01f, "and bounced off it, the speed into it reversed and halved");
            Within(600.0f, fast.VY, 0.01f, "the speed along it kept");

            var slow = ShotAt(Contact.Slide, 580.0f, 900.0f, 100.0f, 4.0f, 600.0f);
            UntilImpacts(world, slow, 1, 64);
            Expect.Equal(1, slow.Impacts.Count, "the slow ball met the wall");
            Within(0.0f, slow.VX, 0.0001f, "the speed into it is gone");
            Within(600.0f, slow.VY, 0.01f, "the speed along it kept");
            var y = slow.Y;
            Steps(world, 16);
            Expect.Equal(1, slow.Impacts.Count, "it slides on with no more impacts");
            Within(584.0f, slow.X, 0.01f, "along the wall");
            Within(y + 300.0f, slow.Y, 0.5f, "at 600 a second");
        }

        /// <summary>A wall's Height and Bounciness below 0 fail in a development build, at Create and when set, and leave it as it was (the checks are left out of a release build, and of the Lua cases); a Height of 0, endless, is allowed.</summary>
        public static void Checks(FakeWorld world)
        {
            Fresh();
            var wall = Wall.Create(0.0f, 0.0f, 100.0f, 0.0f, 50.0f);
#if DEBUG
            Expect.Throws<MoonwellException>(() => Wall.Create(0.0f, 0.0f, 100.0f, 0.0f, -1.0f), "a negative height at Create");
            Expect.Throws<MoonwellException>(() => Wall.Create(0.0f, 0.0f, 100.0f, 0.0f, 0.0f, -0.5f), "a negative bounciness at Create");
            Expect.Throws<MoonwellException>(() => Wall.Create(50.0f, 50.0f, 50.0f, 50.0f), "two ends the same");
            Expect.Throws<MoonwellException>(() => wall.Height = -10.0f, "a negative height set");
            Within(50.0f, wall.Height, 0.0f, "the height kept");
            Expect.Throws<MoonwellException>(() => wall.Bounciness = -1.0f, "a negative bounciness set");
            Within(1.0f, wall.Bounciness, 0.0f, "the bounciness kept");
#endif
            wall.Height = 0.0f;
            Within(0.0f, wall.Height, 0.0f, "0, endless, is allowed");
        }

        /// <summary>A wall 100 high: a ball 200 up flies over it with no contact; one 50 up stops at it.</summary>
        public static void Height(FakeWorld world)
        {
            Fresh();
            var wall = Wall.Create(600.0f, 200.0f, 600.0f, 800.0f, 100.0f);
            Within(100.0f, wall.Height, 0.0f, "its height");
            var high = ShotAt(Contact.Stop, 400.0f, 500.0f, 200.0f, 600.0f, 0.0f);
            var low = ShotAt(Contact.Stop, 400.0f, 600.0f, 50.0f, 600.0f, 0.0f);
            Steps(world, 24);
            Expect.Equal(0, high.Impacts.Count, "the high ball met nothing");
            Expect.True(!high.HasEnded && high.X > 700.0f, "and flew over; at " + high.X);
            Expect.Equal(1, low.Impacts.Count, "the low ball met the wall");
            Expect.True(low.HasEnded, "and stopped");
        }

        /// <summary>At 3,000 a second, more than a ball's width a step, a ball is caught by a wall's end, flying along its line, and by its side, at the point it touched.</summary>
        public static void Fast(FakeWorld world)
        {
            Fresh();
            Wall.Create(600.0f, 300.0f, 600.0f, 700.0f);
            var along = ShotAt(Contact.Stop, 600.0f, 100.0f, 100.0f, 0.0f, 3000.0f);
            var across = ShotAt(Contact.Stop, 100.0f, 520.0f, 100.0f, 3000.0f, 0.0f);
            Steps(world, 16);
            Expect.Equal(1, along.Impacts.Count, "the ball along its line met its end");
            Expect.True(along.HasEnded, "and stopped");
            Within(300.0f, Read(along, 0, 2), 0.01f, "at the end: y");
            Within(600.0f, Read(along, 0, 1), 0.01f, "x");
            Within(-1.0f, Read(along, 0, 5), 0.0001f, "the normal out of the end, back along its line");
            Expect.Equal(1, across.Impacts.Count, "the ball across it met its side");
            Expect.True(across.HasEnded, "and stopped");
            Within(600.0f, Read(across, 0, 1), 0.01f, "at the wall: x");
            Within(520.0f, Read(across, 0, 2), 0.01f, "y");
        }

        /// <summary>
        /// A ball lying against a wall across a slope of 1 in 2, with Gravity 800 and Friction 20, is pulled into the wall by
        /// the slope at every step and goes on down along it, as the guide's loop goes on after a hit for the rest of the
        /// step: a few impacts in its first half second (a bounce off the wall sends a Bounce into the slope), then none; it
        /// is still for no more than one step, and stays on its own side of the wall and on the ground. So rolling
        /// (GroundContact and WallContact Slide), and going along the ground (both Bounce). Left where it met the wall, it
        /// met it again at the start of the next step and stood there.
        /// </summary>
        public static void SlideUnderGravity(FakeWorld world)
        {
            AlongWall(world, Contact.Slide, Contact.Slide, "rolling: ");
            AlongWall(world, Contact.Bounce, Contact.Bounce, "Bounce: ");
        }

        /// <summary>Lays a ball with the given contacts against the wall from (200, 100) to (1400, 1300), on ground that rises 64 a tile of 128 along x, and checks it goes down along the wall for 3 seconds.</summary>
        private static void AlongWall(FakeWorld world, Contact ground, Contact side, string what)
        {
            Interactions.Install(new FakeInteractionWorld());
            Physics.Install(new FakePhysicsWorld());
            var row = "";
            for (var i = 0; i < 16; i++)
            {
                row = row + GridText.Corner(i * 64.0f, 2);
            }

            var text = "";
            for (var i = 0; i < 16; i++)
            {
                text = text + row;
            }

            Terrain.Install(new FakeTerrainWorld(), 16, 16, 0.0f, 0.0f, text);
            Wall.Create(200.0f, 100.0f, 1400.0f, 1300.0f);
            var ball = new Ball();
            ball.GroundContact = ground;
            ball.WallContact = side;
            ball.Gravity = 800.0f;
            ball.Friction = 20.0f;

            // Its radius from the wall's line, y = x - 100, on the side the ground falls toward the wall from.
            var x = 1000.0f + 16.0f * 0.70710678f;
            var y = 900.0f - 16.0f * 0.70710678f;
            Missile.Create(ball, x, y, Physics.GroundHeight(x, y) + 16.0f);
            var still = 0;
            var settled = 0;
            for (var i = 0; i < 96; i++)
            {
                if (i == 16)
                {
                    settled = ball.Impacts;
                }

                var lastX = ball.X;
                var lastY = ball.Y;
                Steps(world, 1);
                if (ball.X == lastX && ball.Y == lastY)
                {
                    still++;
                }

                var off = (ball.X - ball.Y - 100.0f) * 0.70710678f;
                Expect.True(off > 15.99f, what + "on its own side of the wall, its radius off, at step " + i + ": " + off);
                Within(Physics.GroundHeight(ball.X, ball.Y) + 16.0f, ball.Z, 1.0f, what + "on the ground, at step " + i);
            }

            Expect.True(settled <= 3, what + "it met the wall, and the ground after a bounce off it, a few times at first; " + settled + " impacts");
            Expect.Equal(settled, ball.Impacts, what + "then none: it went on along the wall");
            Expect.True(still <= 1, what + "still at " + still + " steps");
            var along = (x - ball.X + y - ball.Y) * 0.70710678f;
            Expect.True(along > 800.0f, what + "went on down along the wall: " + along);
        }

        /// <summary>A footman pushed at 600 into a wall across its way stops short of it, its collision size off, and OnSlammed runs once with the normal out of the wall.</summary>
        public static void Push(FakeWorld world)
        {
            Fresh();
            Wall.Create(650.0f, 100.0f, 650.0f, 1000.0f);
            var handle = world.NewUnit(ObjectTypes.FourCC(PusheeId));
            world.MoveUnit(handle, 512.0f, 512.0f);
            var unit = (Pushee)Unit.Of(handle)!;
            unit.Push(600.0f, 0.0f);
            Steps(world, 64);
            Expect.Equal(1, unit.Slams.Count, "OnSlammed once");
            Within(-1.0f, unit.Slams[0].Normal.X, 0.001f, "the normal out of the wall: x");
            Expect.True(!unit.IsPushed(), "it has stopped");
            Expect.True(unit.X + 16.0f <= 650.0f, "short of the wall, its collision size off; at " + unit.X);
            Expect.True(unit.X > 580.0f, "after moving some way; at " + unit.X);
        }
    }
}
