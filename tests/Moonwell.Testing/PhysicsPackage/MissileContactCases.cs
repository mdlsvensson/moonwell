using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>Cases for two missiles meeting: once a pair, through the one made first, each with its own outcome, the change shared by mass; on flat ground 2,048 across with the default step of 1/32 second, high above it and with no gravity, so the ground is not met.</summary>
    public static class MissileContactCases
    {
        /// <summary>Installs the interactions, physics and terrain packages on fakes, the terrain flat at 0 over 16 by 16 tiles from (0, 0).</summary>
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
            TestLog.Lines.Clear();
        }

        /// <summary>Places a shot with a MissileContact at a place, 100 up, with a level velocity along x, its OnImpact writing its label to the test log, and gives it back.</summary>
        private static Shot ShotAt(string label, Contact contact, float x, float y, float vx)
        {
            var shot = new Shot();
            shot.MissileContact = contact;
            shot.Impacting = impact => TestLog.Lines.Add(label);
            Missile.Create(shot, x, y, 100.0f);
            shot.VX = vx;
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

        /// <summary>The labels the shots' OnImpact wrote, in order, joined with bars; the shots' own lines are left out.</summary>
        private static string Labels()
        {
            var seen = new List<string>();
            for (var i = 0; i < TestLog.Lines.Count; i++)
            {
                var line = TestLog.Lines[i];
                if (line != "shot can-hit" && line != "shot impact")
                {
                    seen.Add(line);
                }
            }

            return string.Join("|", seen);
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
        /// Two bolts head-on at 600 a second each, both with MissileContact Stop: they meet once, halfway, each asked CanHit
        /// about the other and each OnImpact running once, the earlier's first, with the normal out of the other and the
        /// speed they closed at; both end there. Two Pass bolts fly through each other, each meeting the other once.
        /// </summary>
        public static void MeetOnce(FakeWorld world)
        {
            Fresh();
            var first = ShotAt("first", Contact.Stop, 400.0f, 500.0f, 600.0f);
            var second = ShotAt("second", Contact.Stop, 700.0f, 500.0f, -600.0f);
            UntilImpacts(world, first, 1, 64);
            Steps(world, 4);
            Expect.Equal(1, first.Impacts.Count, "the first met once");
            Expect.Equal(1, second.Impacts.Count, "the second met once");
            Expect.Equal("first|second", Labels(), "the earlier's OnImpact first, each once");
            Expect.True(first.HasEnded && second.HasEnded, "both stopped");
            Expect.Same(second, first.Met[0], "the first met the second");
            Expect.Same(first, second.Met[0], "the second met the first");
            Expect.Same(second, first.Impacts[0].Missile, "the impact's Missile");
            Expect.True(first.Impacts[0].Unit == null, "and no unit");
            Expect.True(first.Asked.Count == 1 && first.Asked[0] == second, "the first asked CanHit about the second, once");
            Expect.True(second.Asked.Count == 1 && second.Asked[0] == first, "the second asked about the first, once");
            Within((float)(int)ImpactKind.Missile, Read(first, 0, 0), 0.0f, "of a missile");
            Within(550.0f, Read(first, 0, 1), 0.01f, "halfway: x");
            Within(500.0f, Read(first, 0, 2), 0.01f, "y");
            Within(100.0f, Read(first, 0, 3), 0.01f, "z");
            Within(-1.0f, Read(first, 0, 4), 0.0001f, "the first's normal out of the second: x");
            Within(0.0f, Read(first, 0, 5), 0.0001f, "y");
            Within(1.0f, Read(second, 0, 4), 0.0001f, "the second's normal out of the first: x");
            Within(550.0f, Read(second, 0, 1), 0.01f, "the same point: x");
            Within(1200.0f, Read(first, 0, 7), 0.01f, "the speed they closed at");
            Within(1200.0f, Read(second, 0, 7), 0.01f, "for both");

            // Two that pass through each other, inside each other for steps: still once each, as the pair is checked once.
            var left = ShotAt("left", Contact.Pass, 400.0f, 900.0f, 600.0f);
            var right = ShotAt("right", Contact.Pass, 700.0f, 900.0f, -600.0f);
            Steps(world, 24);
            Expect.Equal(1, left.Impacts.Count, "a Pass met the other once");
            Expect.Equal(1, right.Impacts.Count, "and the other met it once");
            Expect.True(left.X > 700.0f && right.X < 400.0f, "they flew through each other");
            Within(600.0f, left.VX, 0.0f, "at their own speeds");
        }

        /// <summary>
        /// A Stop bolt into a still Bounce shield made before it, neither with mass: the bolt ends, the shield bounces away
        /// with half the change (two with no mass split it evenly), the bounciness the two multiplied and square-rooted (0.5):
        /// 600 * 1.5 / 2 = 450. The shield's OnImpact runs first, as it was made first.
        /// </summary>
        public static void OwnOutcomes(FakeWorld world)
        {
            Fresh();
            var shield = ShotAt("shield", Contact.Bounce, 600.0f, 500.0f, 0.0f);
            var bolt = ShotAt("bolt", Contact.Stop, 400.0f, 500.0f, 600.0f);
            UntilImpacts(world, bolt, 1, 64);
            Expect.Equal(1, bolt.Impacts.Count, "the bolt met the shield");
            Expect.True(bolt.HasEnded, "the bolt stopped");
            Expect.True(!shield.HasEnded, "the shield did not");
            Expect.Equal(1, shield.Impacts.Count, "the shield met the bolt once");
            Expect.Equal("shield|bolt", Labels(), "the shield's OnImpact first");
            Within(450.0f, shield.VX, 0.01f, "the shield bounced away along x");
            Within(0.0f, shield.VY, 0.0001f, "and not across");
            Within(1.0f, Read(shield, 0, 4), 0.0001f, "the shield's normal out of the bolt");
            Steps(world, 8);
            Expect.Equal(1, shield.Impacts.Count, "and met nothing more");
            Expect.True(shield.X > 600.0f, "the shield moved on; at " + shield.X);
        }

        /// <summary>
        /// A Bounce ball lying at rest, its checks paused, hit by a Pass bolt made after it at 300 a second: the bolt's side
        /// checks the pair while the ball rests, and the ball, woken by the hit, checks it after, from the one state the two
        /// share, so each meets the other once while they overlap. Then a bolt whose CanHit refuses the ball: the ball is woken
        /// while they overlap, and the refusal holds, each asked once.
        /// </summary>
        public static void RestThenPass(FakeWorld world)
        {
            Fresh();
            var ball = Resting(world, 500.0f, 500.0f);
            var bolt = new Shot();
            bolt.MissileContact = Contact.Pass;
            bolt.GroundContact = Contact.None;
            Missile.Create(bolt, 300.0f, 500.0f, 24.0f);
            bolt.VX = 300.0f;
            Steps(world, 64);
            Expect.Equal(1, bolt.Impacts.Count, "the bolt met the ball once");
            Expect.Equal(1, ball.Impacts.Count, "the ball met the bolt once");
            Expect.Equal(1, ball.Asked.Count, "the ball was asked once");
            Expect.True(bolt.X > 600.0f, "the bolt flew through; at " + bolt.X);
            Expect.True(ball.X > 500.0f, "the ball was knocked along; at " + ball.X);

            var other = Resting(world, 500.0f, 1000.0f);
            var refused = new Shot();
            refused.MissileContact = Contact.Pass;
            refused.Hit = thing => false;
            refused.GroundContact = Contact.None;
            Missile.Create(refused, 300.0f, 1000.0f, 24.0f);
            refused.VX = 300.0f;
            for (var i = 0; i < 40 && refused.Asked.Count == 0; i++)
            {
                Steps(world, 1);
            }

            Expect.Equal(1, refused.Asked.Count, "the bolt refused the ball");
            other.VX = 100.0f;
            Steps(world, 48);
            Expect.Equal(0, refused.Impacts.Count, "it met nothing");
            Expect.Equal(0, other.Impacts.Count, "nor the ball");
            Expect.Equal(1, refused.Asked.Count, "the refusal held after the ball woke");
            Expect.Equal(1, other.Asked.Count, "and the ball was asked once");
        }

        /// <summary>
        /// Two Stop bolts 100 apart, one creeping at 10 a second: once a fast bolt elsewhere has widened the missile checks'
        /// range, they are paired, and the pair waits for seconds, from the slow speed. Set to 3,000 a second, the creeping
        /// one is met at the other's surface, not flown through, as a velocity set faster ends the waits on it.
        /// </summary>
        public static void SpedUp(FakeWorld world)
        {
            Fresh();
            ShotAt("racer", Contact.Stop, 100.0f, 1500.0f, 3000.0f);
            Steps(world, 2);
            var creeping = ShotAt("creeping", Contact.Stop, 200.0f, 500.0f, 10.0f);
            var still = ShotAt("still", Contact.Stop, 300.0f, 500.0f, 0.0f);
            Steps(world, 4);
            Expect.Equal(0, creeping.Impacts.Count, "nothing met yet");
            creeping.VX = 3000.0f;
            Steps(world, 4);
            Expect.Equal(1, creeping.Impacts.Count, "the sped-up bolt met the other");
            Expect.True(creeping.HasEnded && still.HasEnded, "both stopped");
            Within(284.0f, Read(creeping, 0, 1), 0.01f, "at the other's surface");
        }

        /// <summary>
        /// A ball woken from rest sweeps its two steps (its checks resume a step late) against the other missile's same two
        /// steps, not its last one. A Bounce ball paired with a still Stop bolt rests at (500, 500); it is rolled off at 600
        /// along x as the bolt, 30 behind it and 37.5 above, is sent down at 1,200. In true motion they never come nearer
        /// than about 44, more than their 32: no contact. Swept against the bolt's last step only, the ball would start 30
        /// from it and meet it.
        /// </summary>
        public static void WokenFrame(FakeWorld world)
        {
            Fresh();
            ShotAt("racer", Contact.Stop, 100.0f, 1500.0f, 3000.0f);
            Steps(world, 1);
            var ball = new Shot();
            ball.MissileContact = Contact.Bounce;
            ball.GroundContact = Contact.Slide;
            Missile.Create(ball, 500.0f, 500.0f, 17.0f);
            ball.VZ = -4.0f;
            var bolt = new Shot();
            bolt.MissileContact = Contact.Stop;
            bolt.GroundContact = Contact.None;
            Missile.Create(bolt, 470.0f, 537.5f, 16.0f);
            for (var i = 0; i < 32 && !ball.resting; i++)
            {
                Steps(world, 1);
            }

            Expect.True(ball.resting, "the ball rests");
            Expect.True(ball.meetTouches != null && ball.meetTouches.Count == 1 && ball.meetTouches[0].earlyPair != null, "its pair with the bolt is kept while it rests");
            Within(500.0f, ball.X, 0.0f, "at its place: x");
            ball.VX = 600.0f;
            bolt.VY = -1200.0f;
            Steps(world, 8);
            Expect.Equal(0, ball.Impacts.Count, "the ball met nothing");
            Expect.Equal(0, bolt.Impacts.Count, "nor the bolt");
            Expect.True(!bolt.HasEnded && bolt.Y < 300.0f, "the bolt flew past; at " + bolt.Y);
        }

        /// <summary>A bolt whose MissileContact is None, made first, and a Stop bolt made after it, head-on: the Stop bolt's checks meet it once and it ends; the None bolt reacts to nothing and flies on as it was. Then the same with the Stop bolt made first.</summary>
        public static void NoneSide(FakeWorld world)
        {
            Fresh();
            var quiet = ShotAt("quiet", Contact.None, 400.0f, 500.0f, 600.0f);
            var bolt = ShotAt("bolt", Contact.Stop, 700.0f, 500.0f, -600.0f);
            UntilImpacts(world, bolt, 1, 64);
            Steps(world, 4);
            Expect.Equal(1, bolt.Impacts.Count, "the later Stop bolt met the earlier None bolt once");
            Expect.Same(quiet, bolt.Met[0], "the None bolt");
            Expect.True(bolt.HasEnded, "and stopped");
            Expect.Equal(0, quiet.Impacts.Count, "the None bolt reacted to nothing");
            Expect.Equal(0, quiet.Asked.Count, "and was asked nothing");
            Expect.True(!quiet.HasEnded, "it flies on");
            Within(600.0f, quiet.VX, 0.0f, "as it was");

            var first = ShotAt("first", Contact.Stop, 400.0f, 900.0f, 600.0f);
            var later = ShotAt("later", Contact.None, 700.0f, 900.0f, -600.0f);
            UntilImpacts(world, first, 1, 64);
            Steps(world, 4);
            Expect.Equal(1, first.Impacts.Count, "the earlier Stop bolt met the later None bolt once");
            Expect.Same(later, first.Met[0], "the None bolt");
            Expect.True(first.HasEnded, "and stopped");
            Expect.Equal(0, later.Impacts.Count, "the None bolt reacted to nothing");
            Expect.True(!later.HasEnded, "it flies on");
            Within(-600.0f, later.VX, 0.0f, "as it was");
        }

        /// <summary>Places a Bounce ball on the ground that slides to rest, and steps until it rests.</summary>
        private static Shot Resting(FakeWorld world, float x, float y)
        {
            var ball = new Shot();
            ball.MissileContact = Contact.Bounce;
            ball.GroundContact = Contact.Slide;
            Missile.Create(ball, x, y, 16.0f);
            ball.VZ = -1.0f;
            for (var i = 0; i < 16 && !ball.resting; i++)
            {
                Steps(world, 1);
            }

            Expect.True(ball.resting, "the ball rests");
            Expect.True(ball.missileChecks != null && ball.missileChecks.IsPaused, "its missile checks are paused");
            return ball;
        }

        /// <summary>Two Bounce balls of equal Mass and Bounciness 1, one rolling at 600 into the other, still: their velocities swap. Then the same with one of Mass 0 against one of Mass 3: the one with none takes all the change, bouncing back at 600 and leaving the other still.</summary>
        public static void Mass(FakeWorld world)
        {
            Fresh();
            var moving = ShotAt("moving", Contact.Bounce, 400.0f, 500.0f, 600.0f);
            var still = ShotAt("still", Contact.Bounce, 600.0f, 500.0f, 0.0f);
            moving.Mass = 2.0f;
            still.Mass = 2.0f;
            moving.Bounciness = 1.0f;
            still.Bounciness = 1.0f;
            UntilImpacts(world, moving, 1, 64);
            Expect.Equal(1, moving.Impacts.Count, "they met");
            // Stopped from 600: in the game's own Lua, within 2 parts in 10,000 of the 600 it had.
            Within(0.0f, moving.VX, GameNumbers.HasNaN ? 0.01f : 0.13f, "the moving one stops");
            Within(600.0f, still.VX, 0.01f, "the still one takes its speed");

            var light = ShotAt("light", Contact.Bounce, 400.0f, 1000.0f, 1200.0f);
            var heavy = ShotAt("heavy", Contact.Bounce, 1000.0f, 1000.0f, 0.0f);
            light.Mass = 0.0f;
            heavy.Mass = 3.0f;
            light.Bounciness = 1.0f;
            heavy.Bounciness = 1.0f;
            UntilImpacts(world, light, 1, 64);
            Expect.Equal(1, light.Impacts.Count, "the light one met the heavy one");
            Within(-1200.0f, light.VX, 0.01f, "the one with no mass takes all the change");
            Within(0.0f, heavy.VX, 0.01f, "the one with mass keeps still");
        }
    }
}
