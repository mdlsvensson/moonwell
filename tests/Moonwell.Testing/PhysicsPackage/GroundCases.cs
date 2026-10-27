using System;
using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell.Testing
{
    /// <summary>Cases for a missile meeting the ground: stopping, bouncing, sliding and resting, crests, cliffs, water, flying through, and how often the ground is read; with the default step of 1/32 second.</summary>
    public static class GroundCases
    {
        /// <summary>The height a 10-degree slope climbs over a tile of 128: 128 times the tangent of 10 degrees.</summary>
        private const float SlopeRise = 22.5699f;

        /// <summary>Installs the interactions package and the physics package on fake worlds, and the ground from a grid's text: corners across and up, every 128 from (0, 0). Gives the physics fake.</summary>
        private static FakePhysicsWorld Fresh(int columns, int rows, string corners)
        {
            Interactions.Install(new FakeInteractionWorld());
            var fake = new FakePhysicsWorld();
            Physics.Install(fake);
            Terrain.Install(new FakeTerrainWorld(), columns, rows, 0.0f, 0.0f, corners);
            return fake;
        }

        /// <summary>Installs flat dry ground at a height everywhere.</summary>
        private static FakePhysicsWorld Flat(float height)
        {
            var corner = GridText.Corner(height, 2);
            return Fresh(2, 2, GridText.Tile(corner, corner, corner, corner));
        }

        /// <summary>Installs ground the same at every y, from the heights of its corners every 128 along x, all on one cliff level.</summary>
        private static FakePhysicsWorld Along(float[] heights)
        {
            var row = "";
            for (var i = 0; i < heights.Length; i++)
            {
                row = row + GridText.Corner(heights[i], 2);
            }

            return Fresh(heights.Length, 2, row + row);
        }

        /// <summary>Moves the game on by a number of steps, one step at a time.</summary>
        private static void Steps(FakeWorld world, int count)
        {
            for (var i = 0; i < count; i++)
            {
                world.Advance(Interactions.Step);
            }
        }

        /// <summary>Moves the game on a step at a time, at most a number of steps, until a missile has ended.</summary>
        private static void UntilEnded(FakeWorld world, Missile missile, int most)
        {
            for (var i = 0; i < most && !missile.HasEnded; i++)
            {
                world.Advance(Interactions.Step);
            }
        }

        /// <summary>Fails unless two floats are within a tolerance.</summary>
        private static void Within(float expected, float actual, float tolerance, string what)
        {
            Expect.True(Math.Abs(expected - actual) <= tolerance, what + ": expected " + expected + " within " + tolerance + ", was " + actual);
        }

        /// <summary>
        /// Falling onto flat ground at 50 with the default GroundContact, Stop: OnImpact runs once, with the missile's owner
        /// current and before it ends, then its component's; the impact is of the ground, at the ground's height, with the
        /// normal up and the speed it closed at. It ends there: its effect is moved there before it is destroyed, and its
        /// ImpactModel and ImpactSound play there.
        /// </summary>
        public static void Stop(FakeWorld world)
        {
            var fake = Flat(50.0f);
            var ball = new Ball();
            Expect.True(ball.GroundContact == Contact.Stop, "Stop by default");
            ball.Model = "ball.mdl";
            ball.ImpactModel = "boom.mdl";
            ball.ImpactSound = "boom.wav";
            ball.Gravity = 800.0f;
            ball.VX = 100.0f;
            ball.VZ = -300.0f;
            var log = new List<string>();
            ball.Log = log;
            Missile.Create(ball, 64.0f, 64.0f, 300.0f);
            var tracer = ball.Add(new Tracer(log));
            var look = ball.Effect!;
            UntilEnded(world, ball, 64);
            Expect.True(ball.HasEnded, "it ended");
            Within(50.0f, ball.Z, 0.5f, "it ends at the ground's height");
            Within(ball.X, fake.Places[look][0], 0.001f, "its effect was moved to the point: x");
            Within(50.0f, fake.Places[look][2], 0.5f, "its effect was moved to the point: z");
            Expect.True(fake.Removed.Contains(look), "then destroyed");
            Expect.Equal(1, ball.Impacts, "OnImpact once");
            Within((float)(int)ImpactKind.Ground, ball.Read[0], 0.0f, "an impact with the ground");
            Within(ball.X, ball.Read[1], 0.001f, "the impact's x is the point's");
            Within(64.0f, ball.Read[2], 0.001f, "the impact's y");
            Within(50.0f, ball.Read[3], 0.5f, "the impact's z is the ground's height");
            Within(0.0f, ball.Read[4], 0.001f, "the normal: x");
            Within(0.0f, ball.Read[5], 0.001f, "the normal: y");
            Within(1.0f, ball.Read[6], 0.001f, "the normal: up");
            Expect.True(ball.Read[7] > 300.0f, "the speed it closed at, faster than it started; was " + ball.Read[7]);
            Expect.True(ball.Last!.Contact == Contact.Stop, "the outcome is the setting's");
            Expect.Same(Game.Instance, ball.ImpactedWith, "OnImpact runs with the missile's owner, the game");
            Expect.Same(Game.Instance, tracer.ImpactedWith, "so does the component's");
            Expect.True(!ball.EndedInImpact, "before it ends");
            Expect.Equal(2, log.Count, "two reactions");
            Expect.Equal("ball.impact", log[0], "the missile's first");
            Expect.Equal("tracer.impact", log[1], "then the component's");
            effect? boom = null;
            for (var i = 0; i < fake.Removed.Count; i++)
            {
                if (fake.Models[fake.Removed[i]] == "boom.mdl")
                {
                    boom = fake.Removed[i];
                }
            }

            Expect.True(boom != null, "the ImpactModel was made and destroyed, to play once");
            Within(ball.X, fake.Places[boom!][0], 0.001f, "the ImpactModel at the point: x");
            Within(50.0f, fake.Places[boom!][2], 0.5f, "the ImpactModel at the point: z");
            Expect.Equal(1, fake.Played.Count, "one sound");
            Expect.Equal("boom.wav", fake.Played[0], "the ImpactSound");
        }

        /// <summary>Landing at 400 down on flat ground with Bounce and Bounciness 0.5, it leaves at 200 up within 1, its speed along the ground kept; then it flies on.</summary>
        public static void Bounce(FakeWorld world)
        {
            Flat(0.0f);
            var ball = new Ball();
            ball.GroundContact = Contact.Bounce;
            ball.VX = 100.0f;
            ball.VZ = -400.0f;
            Missile.Create(ball, 64.0f, 64.0f, 100.0f);
            for (var i = 0; i < 32 && ball.VZ < 0.0f; i++)
            {
                Steps(world, 1);
            }

            Expect.Equal(1, ball.Impacts, "OnImpact once");
            Within(400.0f, ball.Read[7], 1.0f, "it closed at 400");
            Within(200.0f, ball.VZ, 1.0f, "it leaves at 200 up");
            Within(100.0f, ball.VX, 0.01f, "its speed along the ground kept");
            Expect.True(!ball.HasEnded, "a bounce does not end it");
            var low = ball.Z;
            Steps(world, 4);
            Expect.True(ball.Z > low + 20.0f, "it flies up after");
            Expect.Equal(1, ball.Impacts, "and meets nothing more");
        }

        /// <summary>With Bounce and Gravity 800, dropped on a slope of 10 degrees, it bounces lower and lower, then goes along the ground down the slope: no impact is slower than 5, and once it has settled there are no more.</summary>
        public static void BounceSettles(FakeWorld world)
        {
            var heights = new float[12];
            for (var i = 0; i < heights.Length; i++)
            {
                heights[i] = i * SlopeRise;
            }

            Along(heights);
            var ball = new Ball();
            ball.GroundContact = Contact.Bounce;
            ball.Gravity = 800.0f;
            ball.VZ = -400.0f;
            Missile.Create(ball, 700.0f, 64.0f, Physics.GroundHeight(700.0f, 64.0f) + 100.0f);
            Steps(world, 64);
            var settled = ball.Impacts;
            Expect.True(settled >= 3, "it bounced; " + settled + " impacts");
            Steps(world, 32);
            Expect.Equal(settled, ball.Impacts, "no impact once it has settled");
            Expect.True(ball.Slowest >= 5.0f, "no impact slower than 5; the slowest was " + ball.Slowest);
            Within(Physics.GroundHeight(ball.X, ball.Y) + 16.0f, ball.Z, 0.5f, "it goes along the ground");
            Expect.True(ball.X < 600.0f, "down the slope; at " + ball.X);
            Expect.True(!ball.HasEnded, "a bounce does not end it");
        }

        /// <summary>
        /// A Bounce lying on a slope of 1 in 2 with Gravity 800 and no velocity goes on down it along the ground, as the guide's
        /// loop goes on after a hit: it meets the ground at the start of every step, and keeps the step's gravity along the
        /// ground. With the step's gravity taken away whole, it stood where it lay.
        /// </summary>
        public static void BounceUnderGravity(FakeWorld world)
        {
            var heights = new float[16];
            for (var i = 0; i < heights.Length; i++)
            {
                heights[i] = i * 64.0f;
            }

            Along(heights);
            var ball = new Ball();
            ball.GroundContact = Contact.Bounce;
            ball.Gravity = 800.0f;
            Missile.Create(ball, 1000.0f, 64.0f, Physics.GroundHeight(1000.0f, 64.0f) + 16.0f);
            for (var i = 0; i < 64; i++)
            {
                Steps(world, 1);
                Within(Physics.GroundHeight(ball.X, ball.Y) + 16.0f, ball.Z, 0.5f, "on the ground, at step " + i);
            }

            Expect.Equal(0, ball.Impacts, "it goes along the ground with no impact");
            Expect.True(1000.0f - ball.X > 300.0f, "slid on down the slope: " + (1000.0f - ball.X) + " along x from where it lay");
            Within(64.0f, ball.Y, 0.001f, "straight down it");
            Expect.True(ball.VX < -300.0f, "and still speeding down it; VX " + ball.VX);
        }

        /// <summary>Before OnImpact runs, VX, VY and VZ read the velocity where it met the ground; the outcome works from the velocity OnImpact leaves, so a Bounce whose reaction sets VZ to 0 does not bounce up.</summary>
        public static void ImpactVelocity(FakeWorld world)
        {
            Flat(0.0f);
            var ball = new Ball();
            ball.GroundContact = Contact.Bounce;
            ball.Gravity = 800.0f;
            ball.VX = 100.0f;
            ball.VZ = -300.0f;
            var seenVX = 0.0f;
            var seenVZ = 0.0f;
            var seenSpeed = 0.0f;
            ball.Impacting = impact =>
            {
                seenVX = ball.VX;
                seenVZ = ball.VZ;
                seenSpeed = impact.Speed;
                ball.VZ = 0.0f;
            };
            Missile.Create(ball, 64.0f, 64.0f, 200.0f);
            for (var i = 0; i < 64 && ball.Impacts == 0; i++)
            {
                Steps(world, 1);
            }

            Expect.Equal(1, ball.Impacts, "it met the ground");
            Within(100.0f, seenVX, 0.01f, "OnImpact reads VX at the point of contact");
            Within(-seenSpeed, seenVZ, 0.01f, "OnImpact reads VZ at the point of contact, the speed it closed at, down");
            Within(0.0f, ball.VZ, 0.01f, "the reaction's VZ of 0 is kept: no bounce up");
            Within(100.0f, ball.VX, 0.01f, "its VX kept");
            Steps(world, 8);
            Within(16.0f, ball.Z, 0.5f, "it goes along the ground");
            Expect.Equal(1, ball.Impacts, "and meets nothing more");
        }

        /// <summary>Moved up by its Z while resting, a ball wakes: it falls, lands, and rests again.</summary>
        public static void WakeMoved(FakeWorld world)
        {
            Flat(0.0f);
            var ball = new Ball();
            ball.GroundContact = Contact.Slide;
            ball.Gravity = 800.0f;
            ball.Friction = 1000.0f;
            Missile.Create(ball, 64.0f, 64.0f, 100.0f);
            for (var i = 0; i < 320 && ball.Rested == 0; i++)
            {
                Steps(world, 1);
            }

            Expect.Equal(1, ball.Rested, "it rests");
            var impacts = ball.Impacts;
            ball.Z = 300.0f;
            Steps(world, 4);
            Expect.True(ball.Z < 300.0f && ball.Z > 200.0f, "it wakes and falls; at " + ball.Z);
            for (var i = 0; i < 320 && ball.Rested == 1; i++)
            {
                Steps(world, 1);
            }

            Expect.True(ball.Impacts > impacts, "it lands");
            Expect.Equal(2, ball.Rested, "and rests again");
            Within(16.0f, ball.Z, 0.5f, "on the ground");
        }

        /// <summary>A missile at 1,500 a second, 60 above flat ground, flying into a ridge 200 high meets the ridge's face, where its lowest point is level with the face, not on top of it.</summary>
        public static void FastRidge(FakeWorld world)
        {
            var heights = new float[14];
            for (var i = 0; i < heights.Length; i++)
            {
                heights[i] = i >= 9 && i <= 11 ? 200.0f : 0.0f;
            }

            Along(heights);
            var ball = new Ball();
            ball.VX = 1500.0f;
            Missile.Create(ball, 124.4f, 64.0f, 60.0f);
            UntilEnded(world, ball, 64);
            Expect.True(ball.HasEnded, "it stopped");
            Expect.Equal(1, ball.Impacts, "OnImpact once");
            var face = 1024.0f + 44.0f / 200.0f * 128.0f;
            Within(face, ball.Read[1], 32.0f, "it met the face");
            Expect.True(ball.Read[3] < 150.0f, "not on top of the ridge; at " + ball.Read[3]);
        }

        /// <summary>
        /// A ball dropped on a slope of 10 degrees with Slide: with Friction 1,000 the slope cannot move it, so it rests on
        /// the ground and OnRest runs once, with the missile current, then its component's; moved, it wakes, and rests
        /// again. With Friction 50 it rolls down the slope and does not rest.
        /// </summary>
        public static void SlideRest(FakeWorld world)
        {
            var heights = new float[12];
            for (var i = 0; i < heights.Length; i++)
            {
                heights[i] = i * SlopeRise;
            }

            Along(heights);
            var ball = new Ball();
            ball.GroundContact = Contact.Slide;
            ball.Gravity = 800.0f;
            ball.Friction = 1000.0f;
            var log = new List<string>();
            ball.Log = log;
            Missile.Create(ball, 700.0f, 64.0f, Physics.GroundHeight(700.0f, 64.0f) + 100.0f);
            var tracer = ball.Add(new Tracer(log));
            for (var i = 0; i < 320 && ball.Rested == 0; i++)
            {
                Steps(world, 1);
            }

            Expect.Equal(1, ball.Rested, "it rests, and OnRest runs");
            Expect.True(ball.Impacts >= 1, "it met the ground first");
            Expect.Same(ball, ball.RestedWith, "OnRest runs with the missile");
            Expect.Same(ball, tracer.RestedWith, "so does the component's");
            Expect.Equal("tracer.rest", log[log.Count - 1], "the component's after the missile's");
            Expect.Equal("ball.rest", log[log.Count - 2], "the missile's first");
            Within(Physics.GroundHeight(ball.X, ball.Y) + 16.0f, ball.Z, 1.0f, "it lies on the ground");
            var restX = ball.X;
            var restZ = ball.Z;
            Steps(world, 64);
            Expect.Equal(1, ball.Rested, "OnRest once");
            Within(restX, ball.X, 0.001f, "it lies still: x");
            Within(restZ, ball.Z, 0.001f, "it lies still: z");
            Expect.True(!ball.HasEnded, "resting does not end it");
            ball.VX = 300.0f;
            Steps(world, 4);
            Expect.True(ball.X > restX + 10.0f, "moved, it wakes and goes");
            for (var i = 0; i < 320 && ball.Rested == 1; i++)
            {
                Steps(world, 1);
            }

            Expect.Equal(2, ball.Rested, "and rests again");
            var roller = new Ball();
            roller.GroundContact = Contact.Slide;
            roller.Gravity = 800.0f;
            roller.Friction = 50.0f;
            Missile.Create(roller, 1000.0f, 64.0f, Physics.GroundHeight(1000.0f, 64.0f) + 100.0f);
            Steps(world, 96);
            Expect.Equal(0, roller.Rested, "with Friction 50 it does not rest");
            Expect.True(roller.X < 900.0f, "it rolls down the slope; was at " + roller.X);
            Expect.True(roller.VX < -50.0f, "and is still rolling down; VX " + roller.VX);
            Within(Physics.GroundHeight(roller.X, roller.Y) + 16.0f, roller.Z, 1.0f, "on the ground");
        }

        /// <summary>Rolling fast up a ridge with Slide, a ball leaves the ground over its crest, where the ground falls away faster than it would fall: it is in the air past it.</summary>
        public static void Crest(FakeWorld world)
        {
            Along(new float[] { 0.0f, 0.0f, 0.0f, 0.0f, 64.0f, 0.0f, 0.0f, 0.0f, 0.0f, 0.0f, 0.0f, 0.0f });
            var ball = new Ball();
            ball.GroundContact = Contact.Slide;
            ball.Gravity = 800.0f;
            ball.VX = 900.0f;
            Missile.Create(ball, 64.0f, 64.0f, 16.0f);
            var rolledBefore = true;
            var airborne = false;
            for (var i = 0; i < 64 && ball.X < 900.0f; i++)
            {
                Steps(world, 1);
                var above = ball.Z - 16.0f - Physics.GroundHeight(ball.X, ball.Y);
                if (ball.X > 100.0f && ball.X < 360.0f && Math.Abs(above) > 0.5f)
                {
                    rolledBefore = false;
                }

                if (ball.X > 530.0f && ball.X < 800.0f && above > 5.0f)
                {
                    airborne = true;
                }
            }

            Expect.True(rolledBefore, "it rolls along the flat before the ridge");
            Expect.True(airborne, "it is in the air past the crest");
            Expect.True(!ball.HasEnded, "and flies on");
        }

        /// <summary>Flying level into a cliff tile with Bounce, a missile bounces back level, as off a wall, not up the cliff's blend: the normal is level.</summary>
        public static void Cliff(FakeWorld world)
        {
            var low0 = GridText.Corner(0.0f, 2);
            var high = GridText.Corner(128.0f, 3);
            var row = low0 + low0 + high + high;
            Fresh(4, 2, row + row);
            Expect.True(Terrain.IsCliff(192.0f, 64.0f), "the tile from 128 to 256 is a cliff");
            var ball = new Ball();
            ball.GroundContact = Contact.Bounce;
            ball.VX = 600.0f;
            Missile.Create(ball, 32.0f, 64.0f, 50.0f);
            for (var i = 0; i < 32 && ball.VX > 0.0f; i++)
            {
                Steps(world, 1);
            }

            Expect.Equal(1, ball.Impacts, "it met the cliff");
            Within(-1.0f, ball.Read[4], 0.001f, "the normal points back: x");
            Within(0.0f, ball.Read[6], 0.001f, "the normal is level");
            Within(-300.0f, ball.VX, 1.0f, "it bounces back at half its speed");
            Within(0.0f, ball.VZ, 0.001f, "level");
            var x = ball.X;
            Steps(world, 4);
            Expect.True(ball.X < x - 30.0f, "and flies back");
            Within(50.0f, ball.Z, 0.01f, "at its height");
        }

        /// <summary>Rolling into a cliff's face with Slide, a ball bounces back off it as off a wall, and that step still counts toward its Duration: a twin whose Duration runs out on that step expires on it.</summary>
        public static void CliffRoll(FakeWorld world)
        {
            var low0 = GridText.Corner(0.0f, 2);
            var high = GridText.Corner(128.0f, 3);
            var row = low0 + low0 + high + high;
            Fresh(4, 2, row + row);
            var ball = new Ball();
            ball.GroundContact = Contact.Slide;
            ball.Gravity = 800.0f;
            ball.VX = 300.0f;
            Missile.Create(ball, 32.0f, 64.0f, 16.0f);
            var wall = 0;
            for (var i = 1; i <= 64 && wall == 0; i++)
            {
                Steps(world, 1);
                if (ball.VX < 0.0f)
                {
                    wall = i;
                }
            }

            Expect.True(wall > 1, "it rolled into the cliff and back; at step " + wall);
            Expect.Equal(1, ball.Impacts, "it met the cliff's face");
            Within(0.0f, ball.Read[6], 0.001f, "as a wall");
            ball.End();
            var twin = new Ball();
            twin.GroundContact = Contact.Slide;
            twin.Gravity = 800.0f;
            twin.VX = 300.0f;
            twin.Duration = wall * Interactions.Step;
            Missile.Create(twin, 32.0f, 64.0f, 16.0f);
            Steps(world, wall - 1);
            Expect.True(!twin.HasEnded, "the twin flies until the wall's step");
            Steps(world, 1);
            Expect.Equal(1, twin.Impacts, "the twin met the wall");
            Expect.True(twin.HasEnded, "and expired on that step");
        }

        /// <summary>With GroundContact None a missile is not checked against the ground: it falls through it, reads nothing, and ends below -2,000.</summary>
        public static void None(FakeWorld world)
        {
            Flat(0.0f);
            var ball = new Ball();
            ball.GroundContact = Contact.None;
            ball.VZ = -1000.0f;
            Missile.Create(ball, 64.0f, 64.0f, 100.0f);
            Ground.reads = 0;
            Steps(world, 4);
            Expect.True(ball.Z < -16.0f && !ball.HasEnded, "it passes below the ground");
            UntilEnded(world, ball, 96);
            Expect.True(ball.HasEnded, "it ended");
            Expect.True(ball.Z < -2000.0f, "below -2,000; was " + ball.Z);
            Expect.Equal(0, ball.Impacts, "no impact");
            Expect.Equal(0, Ground.reads, "the ground was never read");
        }

        /// <summary>A missile 1,000 up falling slowly reads the ground far less than once a step; one that FollowsGround reads it every step.</summary>
        public static void Reads(FakeWorld world)
        {
            Flat(0.0f);
            var ball = new Ball();
            ball.VZ = -100.0f;
            ball.VX = 50.0f;
            Missile.Create(ball, 64.0f, 64.0f, 1000.0f);
            Ground.reads = 0;
            Steps(world, 64);
            Expect.True(!ball.HasEnded, "still falling");
            Expect.True(Ground.reads >= 1, "the ground was read; " + Ground.reads + " reads");
            Expect.True(Ground.reads <= 4, "far less than once a step; " + Ground.reads + " reads in 64 steps");
            var follower = new Ball();
            follower.FollowsGround = true;
            follower.VX = 300.0f;
            Missile.Create(follower, 64.0f, 128.0f, 60.0f);
            ball.End();
            Ground.reads = 0;
            Steps(world, 64);
            Expect.True(Ground.reads >= 64, "FollowsGround reads every step; " + Ground.reads + " reads in 64 steps");
        }

        /// <summary>Falling onto water, a missile stops on its surface, as on ground: the grid's height there is the water's.</summary>
        public static void Water(FakeWorld world)
        {
            var wet = GridText.Corner(-20.0f, 2, false, true);
            Fresh(2, 2, GridText.Tile(wet, wet, wet, wet));
            var ball = new Ball();
            ball.VZ = -500.0f;
            Missile.Create(ball, 64.0f, 64.0f, 200.0f);
            UntilEnded(world, ball, 64);
            Expect.True(ball.HasEnded, "it stopped");
            Within(-20.0f, ball.Z, 0.5f, "on the water's surface");
            Expect.Equal(1, ball.Impacts, "one impact");
            Within((float)(int)ImpactKind.Ground, ball.Read[0], 0.0f, "with the ground");
        }

        /// <summary>
        /// A read put off is made, from the grid it was put off on, by whichever needs its height first. Three balls fly
        /// along x at 900 over ground that climbs 3 every 128 along x, their bottoms at 314, above the 301 of their span (the
        /// block's highest corner, 300, plus 1): the same ground under each, so their reads fall due on the same steps.
        /// A plain one's read is put off, and the slack check of the next step, its bottom 13 above spanHigh and 315 above
        /// spanLow, falls between the two and makes it. One with a shadow has it made by the shadow in that same step. One
        /// turned to follow the ground has it made as the ground under it before its move.
        /// </summary>
        public static void PutOff(FakeWorld world)
        {
            var corners = "";
            for (var row = 0; row < 8; row++)
            {
                for (var column = 0; column < 20; column++)
                {
                    corners = corners + GridText.Corner(column == 12 && row == 7 ? 300.0f : column * 3.0f, 2);
                }
            }

            Fresh(20, 8, corners);
            var plain = new Ball();
            var shaded = new Ball();
            var follower = new Ball();
            var balls = new Ball[] { plain, shaded, follower };
            for (var k = 0; k < balls.Length; k++)
            {
                balls[k].VX = 900.0f;
                Missile.Create(balls[k], 200.0f, 192.0f + k * 256.0f, 330.0f);
            }

            var shadow = shaded.Add(new Shadow());
            var steps = 0;
            while (!plain.groundPending && steps < 16)
            {
                world.Advance(Interactions.Step);
                steps++;
            }

            Expect.True(plain.groundPending, "the plain ball's read is put off within 16 steps; " + steps + " steps");
            Expect.True(follower.groundPending, "and the follower's, on the same step");
            Within(301.0f, plain.spanHigh, 0.0f, "the span's high is the block's highest corner plus 1");
            Within(-1.0f, plain.spanLow, 0.0f, "its low the lowest less 1");

            // The shadow made its ball's read in the step it was put off, at the ball's place.
            Expect.True(!shaded.groundPending && shaded.groundRead, "the shadow made its ball's read");
            Expect.True(shaded.groundX == shaded.X && shaded.groundY == shaded.Y, "at the ball's place");
            Expect.True(shaded.groundZ == Terrain.Height(shaded.X, shaded.Y), "the read is the ground there: " + shaded.groundZ);
            Expect.True(shadow.groundZ == shaded.groundZ, "and the shadow has it");

            // The follower's ground before its move is the put-off read, at its place.
            follower.FollowsGround = true;
            var fromX = follower.X;
            var fromZ = follower.Z;
            var stale = follower.groundZ;
            var below = Terrain.Height(fromX, follower.Y);
            Expect.True(stale != below, "the field still holds an older read, so a stale one would show");
            var plainX = plain.groundX;
            var plainY = plain.groundY;
            world.Advance(Interactions.Step);
            var toX = fromX + 900.0f * Interactions.Step;
            Within(toX, follower.X, 0.001f, "the follower moved along x");
            Within(Terrain.Height(toX, follower.Y) + fromZ - below, follower.Z, 0.001f, "kept its height above the ground, from the put-off read");
            Expect.True(!follower.groundPending && follower.groundZ == Terrain.Height(follower.X, follower.Y), "and read the ground where it is");

            // The plain ball's slack check made its read: where it was put off, as the ground there.
            Expect.True(!plain.groundPending, "the slack check made the plain ball's read");
            Expect.True(plain.groundX == plainX && plain.groundY == plainY, "at the place it was put off");
            Expect.True(plain.groundZ == Terrain.Height(plainX, plainY), "as the ground there: " + plain.groundZ);
            Expect.True(!plain.HasEnded && plain.Z == 330.0f && plain.Impacts == 0, "and it flies on, level");
        }
    }
}
