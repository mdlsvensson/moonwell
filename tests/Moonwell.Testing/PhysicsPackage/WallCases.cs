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

        /// <summary>Fails unless two floats are within a tolerance; in the game's own Lua, whose square roots and trig are off by up to about 2 parts in 100,000, within it and 2 parts in 10,000 of the expected value, as Expect.Near.</summary>
        private static void Within(float expected, float actual, float tolerance, string what)
        {
            var allowed = GameNumbers.HasNaN ? tolerance : tolerance + Math.Abs(expected) * 0.0002f;
            Expect.True(Math.Abs(expected - actual) <= allowed, what + ": expected " + expected + " within " + allowed + ", was " + actual);
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
            var maker = Scope.Current;
            var wall = Wall.Create(600.0f, 200.0f, 600.0f, 800.0f);
            Expect.Same(maker, wall.Parent, "it belongs to its maker");
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
            var keeper = new TestScope("keeper", log);
            var soft = Wall.Create(600.0f, 1200.0f, 600.0f, 1800.0f, 0.0f, 0.5f, keeper);
            Expect.Same(keeper, soft.Parent, "a wall given an owner belongs to it");
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
            // Gone from 4: in the game's own Lua, within 2 parts in 10,000 of the 4 it had.
            Within(0.0f, slow.VX, GameNumbers.HasNaN ? 0.0001f : 0.0009f, "the speed into it is gone");
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

        /// <summary>Installs the packages on fakes with ground that rises 64 a tile of 128 along x, 16 by 16 tiles from (0, 0), and the wall from (200, 100) to (1400, 1300) across it.</summary>
        private static void SlopeWithWall()
        {
            SlopeWithWall(0.0f);
        }

        /// <summary>As SlopeWithWall, the ground, the wall and the fakes' bounds moved by shift along x and along y.</summary>
        private static void SlopeWithWall(float shift)
        {
            var interactions = new FakeInteractionWorld();
            interactions.MinX += shift;
            interactions.MinY += shift;
            interactions.MaxX += shift;
            interactions.MaxY += shift;
            Interactions.Install(interactions);
            var physics = new FakePhysicsWorld();
            physics.MinX += shift;
            physics.MinY += shift;
            physics.MaxX += shift;
            physics.MaxY += shift;
            Physics.Install(physics);
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

            Terrain.Install(new FakeTerrainWorld(), 16, 16, shift, shift, text);
            Wall.Create(200.0f + shift, 100.0f + shift, 1400.0f + shift, 1300.0f + shift);
        }

        /// <summary>Lays a ball with the given contacts, Gravity 800 and Friction 20 against the slope's wall, its radius from the wall's line, y = x - 100, on the side the ground falls toward the wall from, and gives it back.</summary>
        private static Ball AgainstSlopeWall(Contact ground, Contact side, out float x, out float y)
        {
            var ball = new Ball();
            ball.GroundContact = ground;
            ball.WallContact = side;
            ball.Gravity = 800.0f;
            ball.Friction = 20.0f;
            x = 1000.0f + 16.0f * 0.70710678f;
            y = 900.0f - 16.0f * 0.70710678f;
            Missile.Create(ball, x, y, Physics.GroundHeight(x, y) + 16.0f);
            return ball;
        }

        /// <summary>Lays a ball with the given contacts against the wall from (200, 100) to (1400, 1300), on ground that rises 64 a tile of 128 along x, and checks it goes down along the wall for 3 seconds.</summary>
        private static void AlongWall(FakeWorld world, Contact ground, Contact side, string what)
        {
            SlopeWithWall();
            float x;
            float y;
            var ball = AgainstSlopeWall(ground, side, out x, out y);
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

            Expect.True(settled <= (GameNumbers.HasNaN ? 3 : 8), what + "it met the wall, and the ground after a bounce off it, a few times at first; " + settled + " impacts");
            Expect.Equal(settled, ball.Impacts, what + "then none: it went on along the wall");
            Expect.True(still <= 1, what + "still at " + still + " steps");
            var along = (x - ball.X + y - ball.Y) * 0.70710678f;
            Expect.True(along > 800.0f, what + "went on down along the wall: " + along);
        }

        /// <summary>
        /// The slide-under-gravity scene with a second wall from (300, 200) to (600, -100) across the ball's way, closing the
        /// corner at (300, 200): the ball comes down along the first wall at about 640 a second, more than its radius a step,
        /// and stays its radius off both walls, and on the ground: rolling throughout, and as a Bounce, knocked back up the
        /// slope by the second wall to bounce on it a few times, once settled. In the corner it comes to rest: no impact
        /// after 8 s (the Bounce's last near 6 s), OnRest once, and still for the last second of 16. The slide along the
        /// first wall ended the step's walk down the walls, so the second was met a step late, from its far side, and the
        /// ball went through it; then, held in the corner, it was put back out uphill into the ground, so a Bounce met the
        /// ground at 5.2 every 3 steps for ever, and neither ever rested.
        /// </summary>
        public static void SlideIntoCorner(FakeWorld world)
        {
            IntoCorner(world, Contact.Slide, Contact.Slide, "rolling: ");
            IntoCorner(world, Contact.Bounce, Contact.Bounce, "Bounce: ");
        }

        /// <summary>Lays a ball with the given contacts against the slope's wall with the second wall across its way, and checks it stays off both and on the ground, and rests in the corner.</summary>
        private static void IntoCorner(FakeWorld world, Contact ground, Contact side, string what)
        {
            SlopeWithWall();
            Wall.Create(300.0f, 200.0f, 600.0f, -100.0f);
            float x;
            float y;
            var ball = AgainstSlopeWall(ground, side, out x, out y);
            var fastest = 0.0f;
            var settled = 0;
            var lastX = 0.0f;
            var lastY = 0.0f;
            for (var i = 0; i < 512; i++)
            {
                if (i == 256)
                {
                    settled = ball.Impacts;
                }

                if (i == 480)
                {
                    lastX = ball.X;
                    lastY = ball.Y;
                }

                Steps(world, 1);
                var speed = MathF.Sqrt(ball.VX * ball.VX + ball.VY * ball.VY);
                if (speed > fastest)
                {
                    fastest = speed;
                }

                var offA = (ball.X - ball.Y - 100.0f) * 0.70710678f;
                var offB = (ball.X + ball.Y - 500.0f) * 0.70710678f;
                Expect.True(offA > 15.9f, what + "its radius off the first wall, at step " + i + ": " + offA);
                Expect.True(offB > 15.9f, what + "its radius off the second wall, at step " + i + ": " + offB);
                var floor = Physics.GroundHeight(ball.X, ball.Y) + 16.0f;
                if (ground == Contact.Slide || i >= 256)
                {
                    Within(floor, ball.Z, 1.0f, what + "on the ground, at step " + i);
                }
                else
                {
                    // A Bounce knocked back up the slope by the second wall at 640 a second meets the slope and bounces
                    // off it a few times before it settles.
                    Expect.True(ball.Z > floor - 1.0f, what + "not under the ground, at step " + i + ": " + (ball.Z - floor));
                }
            }

            Expect.True(fastest > 16.0f * 32.0f, what + "faster than its radius a step on the way: " + fastest);
            Expect.True(ball.X < 360.0f && ball.Y < 240.0f, what + "in the corner at the end; at (" + ball.X + ", " + ball.Y + ")");
            Expect.Equal(settled, ball.Impacts, what + "no impact after 8 s");
            Expect.Equal(1, ball.Rested, what + "it rests in the corner, OnRest once");
            Within(lastX, ball.X, 0.0f, what + "still for the last second: x");
            Within(lastY, ball.Y, 0.0f, what + "y");
        }

        /// <summary>
        /// Corners wider than a right angle on the slope of 1 in 2: two walls 500 long meet at (800, 800), opening uphill,
        /// 135 and 160 degrees apart. A ball with Friction 20 laid inside, in the middle or off to one side, rolling or a
        /// Bounce, comes down into the corner and rests there: OnRest once, no impact after 8 s, still for the last second
        /// of 16. The rest test took the velocity the walk left and took it and the pull along one wall's foot after
        /// another, which leaves a share moving along the second wall in any corner but a right angle: such a ball never
        /// rested, and a Bounce met the walls and the ground 5 to 25 times a second.
        /// </summary>
        public static void WideCorners(FakeWorld world)
        {
            var halves = new float[] { 67.5f, 80.0f };
            var contacts = new Contact[] { Contact.Slide, Contact.Bounce };
            for (var h = 0; h < halves.Length; h++)
            {
                for (var c = 0; c < contacts.Length; c++)
                {
                    for (var o = 0; o < 2; o++)
                    {
                        var what = "V" + (2.0f * halves[h]) + " " + contacts[c] + (o == 0 ? " in the middle: " : " to one side: ");
                        SlopeOnly();
                        var a = halves[h] * 0.0174532925f;
                        var ca = MathF.Cos(a);
                        var sa = MathF.Sin(a);
                        Wall.Create(800.0f, 800.0f, 800.0f + 500.0f * ca, 800.0f + 500.0f * sa);
                        Wall.Create(800.0f, 800.0f, 800.0f + 500.0f * ca, 800.0f - 500.0f * sa);
                        var x = 800.0f + 150.0f * ca;
                        var y = 800.0f + (o == 0 ? 0.0f : 150.0f * sa - 25.0f / ca);
                        var ball = new Ball();
                        ball.GroundContact = contacts[c];
                        ball.WallContact = contacts[c];
                        ball.Gravity = 800.0f;
                        ball.Friction = 20.0f;
                        Missile.Create(ball, x, y, Physics.GroundHeight(x, y) + 16.0f);
                        var settled = 0;
                        var lastX = 0.0f;
                        var lastY = 0.0f;
                        for (var i = 0; i < 512; i++)
                        {
                            if (i == 256)
                            {
                                settled = ball.Impacts;
                            }

                            if (i == 480)
                            {
                                lastX = ball.X;
                                lastY = ball.Y;
                            }

                            Steps(world, 1);
                        }

                        Expect.Equal(1, ball.Rested, what + "it rests in the corner, OnRest once");
                        Expect.Equal(settled, ball.Impacts, what + "no impact after 8 s");
                        Within(lastX, ball.X, 0.0f, what + "still for the last second: x");
                        Within(lastY, ball.Y, 0.0f, what + "y");
                        Within(Physics.GroundHeight(ball.X, ball.Y) + 16.0f, ball.Z, 1.0f, what + "on the ground");
                    }
                }
            }
        }

        /// <summary>
        /// A ball resting against a wall across the slope of 1 in 2, which its Friction (20 or 200) cannot hold on its own,
        /// wakes when the wall goes: ended, it goes on down the slope; moved 300 down the slope, it goes on down to the wall
        /// where it is now and rests against it again. So rolling and as a Bounce. It lay still where the wall had been.
        /// </summary>
        public static void WallGoes(FakeWorld world)
        {
            var contacts = new Contact[] { Contact.Slide, Contact.Bounce };
            var frictions = new float[] { 20.0f, 200.0f };
            for (var c = 0; c < contacts.Length; c++)
            {
                for (var f = 0; f < frictions.Length; f++)
                {
                    var what = contacts[c] + " with Friction " + frictions[f] + ": ";
                    SlopeOnly();
                    var wall = Wall.Create(600.0f, 0.0f, 600.0f, 2000.0f);
                    var ball = Resting(world, contacts[c], frictions[f]);
                    Expect.Equal(1, ball.Rested, what + "it rests against the wall");
                    var restX = ball.X;
                    wall.End();
                    Steps(world, 64);
                    Expect.True(ball.X < restX - 100.0f, what + "the wall ended, it goes on down the slope; at " + ball.X + " from " + restX);

                    SlopeOnly();
                    var moved = Wall.Create(600.0f, 0.0f, 600.0f, 2000.0f);
                    var other = Resting(world, contacts[c], frictions[f]);
                    Expect.Equal(1, other.Rested, what + "the other rests against the wall");
                    moved.X = moved.X - 300.0f;
                    for (var i = 0; i < 320 && other.Rested < 2; i++)
                    {
                        Steps(world, 1);
                    }

                    Expect.Equal(2, other.Rested, what + "the wall moved, it goes on down to it and rests again");
                    Within(316.0f, other.X, 1.0f, what + "against the wall where it is now, its radius off");
                }
            }
        }

        /// <summary>
        /// A ball resting against a wall across the slope of 1 in 2 is woken once by the wall moving away from it, not at
        /// every set of the wall's place: the wall's X set to itself every step for 8 s leaves it resting (OnRest once);
        /// then moved 400 down the slope, it goes on down to the wall and rests against it again (OnRest twice), and the
        /// wall's X set to itself every step for 4 s more leaves it so. A wall moved 10 into a resting ball does not wake
        /// it: the wall goes into it, as walls do not push missiles. So rolling and as a Bounce. It got OnRest at every set, 16 or 32 a second, for ever.
        /// </summary>
        public static void WallSet(FakeWorld world)
        {
            var contacts = new Contact[] { Contact.Slide, Contact.Bounce };
            for (var c = 0; c < contacts.Length; c++)
            {
                var what = contacts[c] + ": ";
                SlopeOnly();
                var wall = Wall.Create(600.0f, 0.0f, 600.0f, 2000.0f);
                var ball = Resting(world, contacts[c], 200.0f);
                Expect.Equal(1, ball.Rested, what + "it rests against the wall");
                for (var i = 0; i < 256; i++)
                {
                    wall.X = wall.X;
                    Steps(world, 1);
                }

                Expect.Equal(1, ball.Rested, what + "the wall's X set to itself every step for 8 s: OnRest still once");
                wall.X = wall.X - 400.0f;
                for (var i = 0; i < 320 && ball.Rested < 2; i++)
                {
                    Steps(world, 1);
                }

                Expect.Equal(2, ball.Rested, what + "the wall moved away, it goes on down to it and rests again");
                Within(216.0f, ball.X, 1.0f, what + "against the wall where it is now, its radius off");
                for (var i = 0; i < 128; i++)
                {
                    wall.X = wall.X;
                    Steps(world, 1);
                }

                Expect.Equal(2, ball.Rested, what + "and set to itself for 4 s more: OnRest still twice");

                var restX = ball.X;
                for (var i = 0; i < 20; i++)
                {
                    wall.X = wall.X + 0.5f;
                    Steps(world, 1);
                }

                Expect.Equal(2, ball.Rested, what + "a wall moved 10 into it does not wake it");
                Within(restX, ball.X, 0.0f, what + "it lies still, the wall in it");
            }
        }

        /// <summary>Lays a ball with a contact and a Friction at (640, 1000) on the slope, above the wall at x 600, and steps until it rests against it, at most 3 s.</summary>
        private static Ball Resting(FakeWorld world, Contact contact, float friction)
        {
            var ball = new Ball();
            ball.GroundContact = contact;
            ball.WallContact = contact;
            ball.Gravity = 800.0f;
            ball.Friction = friction;
            Missile.Create(ball, 640.0f, 1000.0f, Physics.GroundHeight(640.0f, 1000.0f) + 16.0f);
            for (var i = 0; i < 96 && ball.Rested == 0; i++)
            {
                Steps(world, 1);
            }

            return ball;
        }

        /// <summary>Installs the packages on fakes with the slope of SlopeWithWall, rising 64 a tile of 128 along x, and no wall.</summary>
        private static void SlopeOnly()
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
        }

        /// <summary>
        /// A ball of radius 0, rolling or a Bounce, laid 1 off the slope's wall on the side the ground falls toward it from,
        /// is pulled into the wall at every step and goes on down along it for 3 seconds on its own side: put back out at
        /// the wall's side whenever its middle is on or past the wall's line, whatever its radius, and left just off the
        /// line after a bounce off it. With radius 0 nothing was put back, and it went through the wall.
        /// </summary>
        public static void SlideRadiusZero(FakeWorld world)
        {
            AlongWallThin(world, Contact.Slide, Contact.Slide, "rolling: ", 0.0f);
            AlongWallThin(world, Contact.Bounce, Contact.Bounce, "Bounce: ", 0.0f);
        }

        /// <summary>
        /// As SlideRadiusZero, 12,288 from the world's origin along x and y, where a float's last bits are about 0.001: a
        /// thin ball meets the wall with its middle on the line, and its normal was taken from how far its middle was from
        /// the wall's nearest point, which was only those bits there (and in the game's own Lua at 1,000, whose additions drop
        /// more of them), so it pointed any way and the ball went through the wall. On the wall's straight part the normal is
        /// the wall's own.
        /// </summary>
        public static void SlideRadiusZeroFar(FakeWorld world)
        {
            AlongWallThin(world, Contact.Slide, Contact.Slide, "rolling: ", 12288.0f);
            AlongWallThin(world, Contact.Bounce, Contact.Bounce, "Bounce: ", 12288.0f);
        }

        /// <summary>Lays a ball of radius 0 with the given contacts 1 off the slope's wall, the slope and the wall moved by shift along x and y, and checks it stays on its own side for 3 seconds.</summary>
        private static void AlongWallThin(FakeWorld world, Contact ground, Contact side, string what, float shift)
        {
            SlopeWithWall(shift);
            var ball = new Ball();
            ball.GroundContact = ground;
            ball.WallContact = side;
            ball.Gravity = 800.0f;
            ball.Friction = 20.0f;
            ball.Radius = 0.0f;
            var x = 1000.0f + 0.70710678f + shift;
            var y = 900.0f - 0.70710678f + shift;
            Missile.Create(ball, x, y, Physics.GroundHeight(x, y));
            var least = 1.0f;
            for (var i = 0; i < 96; i++)
            {
                Steps(world, 1);
                // Across the wall, from the wall's line: not past it, nor on it.
                var off = ball.X - ball.Y - 100.0f;
                if (off < least)
                {
                    least = off;
                }
            }

            Expect.True(least > 0.0f, what + "on its own side of the wall throughout; at least " + least);
            var along = (x - ball.X + y - ball.Y) * 0.70710678f;
            Expect.True(along > 400.0f, what + "went on down along the wall: " + along);
        }

        /// <summary>
        /// A Pass ball whose OnImpact moves it, at its first meeting with a long wall, to another part of the same wall's
        /// near side meets the wall again where it comes to it there: what it kept of the wall is forgotten when a reaction
        /// moves it, as when it is moved from outside. The touch kept as passed skipped the second meeting.
        /// </summary>
        public static void JumpedAlong(FakeWorld world)
        {
            Fresh();
            Wall.Create(600.0f, 100.0f, 600.0f, 2000.0f);
            var shot = ShotAt(Contact.Pass, 400.0f, 200.0f, 100.0f, 600.0f, 0.0f);
            var jumped = false;
            shot.Impacting = impact =>
            {
                if (!jumped)
                {
                    jumped = true;
                    shot.X = 400.0f;
                    shot.Y = 1500.0f;
                }
            };

            Steps(world, 32);
            Expect.Equal(2, shot.Impacts.Count, "it met the wall where it was moved to as well");
            Within(1500.0f, Read(shot, 1, 2), 0.01f, "there: y");
            Within(600.0f, Read(shot, 1, 1), 0.01f, "at the wall: x");
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

        /// <summary>
        /// A pushed unit's line ahead through the corner where two walls meet is stopped, whatever a float's rounding, as each
        /// wall reaches 0.01 past its ends (as Raycast's walls): this line from (1094, 568), through the corner (976, 607)
        /// into the walls' angle, missed both walls in floats by their last bit. A line straight up 0.005 beside the corner,
        /// outside the walls' angle, is stopped; one 0.05 beside it is not.
        /// </summary>
        public static void PushCorner(FakeWorld world)
        {
            Fresh();
            Wall.Create(711.0f, 421.0f, 976.0f, 607.0f);
            Wall.Create(976.0f, 607.0f, 748.0f, 933.0f);
            var handle = world.NewUnit(ObjectTypes.FourCC(PusheeId));
            world.MoveUnit(handle, 1094.0f, 568.0f);
            var push = new Engine.Pushed(Unit.Of(handle)!, false, 16.0f);
            Expect.True(Engine.Walls.Blocks(push, 1094.0f, 568.0f, 893.4f, 634.3f), "through the corner: stopped");
            Expect.True(Engine.Walls.Blocks(push, 976.005f, 500.0f, 976.005f, 700.0f), "0.005 beside the corner: stopped");
            Expect.True(!Engine.Walls.Blocks(push, 976.05f, 500.0f, 976.05f, 700.0f), "0.05 beside it: not stopped");
        }

        /// <summary>
        /// A unit standing on a wall (a wall made through its place) is not held by it, whichever way it is pushed, as a
        /// raycast from the surface it leaves is no hit: pushed at 600 north or south, it moves off the wall with no slam. It
        /// was stopped at once both ways, the wall crossing its line at its start.
        /// </summary>
        public static void PushOnWall(FakeWorld world)
        {
            Fresh();
            Wall.Create(300.0f, 512.0f, 700.0f, 512.0f);
            var north = world.NewUnit(ObjectTypes.FourCC(PusheeId));
            world.MoveUnit(north, 512.0f, 512.0f);
            var northUnit = (Pushee)Unit.Of(north)!;
            northUnit.Push(0.0f, 600.0f);
            var south = world.NewUnit(ObjectTypes.FourCC(PusheeId));
            world.MoveUnit(south, 600.0f, 512.0f);
            var southUnit = (Pushee)Unit.Of(south)!;
            southUnit.Push(0.0f, -600.0f);
            Steps(world, 16);
            Expect.Equal(0, northUnit.Slams.Count + southUnit.Slams.Count, "no slam");
            Expect.True(northUnit.Y > 600.0f, "pushed north, it moved off the wall; at " + northUnit.Y);
            Expect.True(southUnit.Y < 424.0f, "pushed south, so did the other; at " + southUnit.Y);
        }

        /// <summary>
        /// Forty units 0.011 to 0.05 north of a wall along y 512, pushed at (600, -0.3), glancing toward it at 0.03 degrees,
        /// do not cross it: a unit within 0.01 of a wall is let go only where its line ends on its own side of the wall, so
        /// one that has drifted that near is stopped by it, not let through. Eight of them ended on the far side.
        /// </summary>
        public static void PushGlancing(FakeWorld world)
        {
            Fresh();
            Wall.Create(300.0f, 512.0f, 2000.0f, 512.0f);
            var units = new List<Pushee>();
            for (var i = 0; i < 40; i++)
            {
                var handle = world.NewUnit(ObjectTypes.FourCC(PusheeId));
                world.MoveUnit(handle, 400.0f + i * 20.0f, 512.0f + 0.011f + i * 0.001f);
                var unit = (Pushee)Unit.Of(handle)!;
                unit.Push(600.0f, -0.3f);
                units.Add(unit);
            }

            Steps(world, 48);
            for (var i = 0; i < units.Count; i++)
            {
                Expect.True(!units[i].IsPushed(), "unit " + i + " has stopped");
                Expect.True(units[i].Y >= 512.0f, "unit " + i + " on its own side of the wall; at y " + units[i].Y);
            }
        }
    }
}
