using System;
using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell.Testing
{
    /// <summary>Cases for a unit's push: knockback, blocking and sliding on pathable ground, lifts and landings, flyers, overlapping pushes, ending, the order and the grid, Body, and the reactions; on flat ground 2,048 across with the default step of 1/32 second.</summary>
    public static class PushCases
    {
        /// <summary>The type id the Pushee class is registered with.</summary>
        private const string PusheeId = "h0P1";

        /// <summary>Installs the interactions, physics and terrain packages on fakes, the terrain flat at 0 over 16 by 16 tiles from (0, 0), and registers Pushee. Gives the physics fake.</summary>
        private static FakePhysicsWorld Fresh(out FakeTerrainWorld terrain, out FakeInteractionWorld interactions)
        {
            interactions = new FakeInteractionWorld();
            Interactions.Install(interactions);
            var fake = new FakePhysicsWorld();
            Physics.Install(fake);
            terrain = new FakeTerrainWorld();
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

            Terrain.Install(terrain, 17, 17, 0.0f, 0.0f, text);
            ObjectTypes.Unit(ObjectTypes.FourCC(PusheeId), () => new Pushee());
            return fake;
        }

        /// <summary>Makes a Pushee at a place, and takes it in.</summary>
        private static Pushee PusheeAt(FakeWorld world, float x, float y)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC(PusheeId));
            world.MoveUnit(handle, x, y);
            return (Pushee)Unit.Of(handle)!;
        }

        /// <summary>Makes a footman of the game at a place, and takes it in.</summary>
        private static Unit FootmanAt(FakeWorld world, float x, float y)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            world.MoveUnit(handle, x, y);
            return Unit.Of(handle)!;
        }

        /// <summary>Makes the column of 32 by 32 cells that holds an x unwalkable (or unflyable), from y 0 to 2,048: a line square across x.</summary>
        private static void LineAt(FakeTerrainWorld terrain, float x, int flags)
        {
            for (var y = 16.0f; y < 2048.0f; y += 32.0f)
            {
                terrain.Block(x, y, flags);
            }
        }

        /// <summary>Makes the row of 32 by 32 cells that holds a y unwalkable, from x 0 to 2,048: a line square across y.</summary>
        private static void RowAt(FakeTerrainWorld terrain, float y, int flags)
        {
            for (var x = 16.0f; x < 2048.0f; x += 32.0f)
            {
                terrain.Block(x, y, flags);
            }
        }

        /// <summary>Moves the game on by a number of steps, one step at a time.</summary>
        private static void Steps(FakeWorld world, int count)
        {
            for (var i = 0; i < count; i++)
            {
                world.Advance(Interactions.Step);
            }
        }

        /// <summary>The speed of a unit's push along the ground.</summary>
        private static float Speed(Unit unit)
        {
            var velocity = unit.PushVelocity();
            return MathF.Sqrt(velocity.X * velocity.X + velocity.Y * velocity.Y);
        }

        /// <summary>How many times the core fake put a unit somewhere with SetUnitX/Y.</summary>
        private static int Places(FakeWorld world)
        {
            var count = 0;
            for (var i = 0; i < world.UnitCalls.Count; i++)
            {
                if (world.UnitCalls[i] == "place")
                {
                    count++;
                }
            }

            return count;
        }

        /// <summary>Fails unless two floats are within a tolerance.</summary>
        private static void Within(float expected, float actual, float tolerance, string what)
        {
            Expect.True(Math.Abs(expected - actual) <= tolerance, what + ": expected " + expected + " within " + tolerance + ", was " + actual);
        }

        /// <summary>Knockback 300 over half a second: 1,200 a second along the angle, slowing evenly; it ends 300 away within 4 at 0.5 s, within a step, with no OnSlammed. OnPushed runs once, with no pusher and the unit current. The cycle's timer runs for it with nothing tracked, and stops once it ends.</summary>
        public static void Knockback(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            FakeInteractionWorld interactions;
            Fresh(out terrain, out interactions);
            var unit = PusheeAt(world, 512.0f, 512.0f);
            Expect.True(!Tracking.Running, "nothing runs the cycle before the push");
            unit.Knockback(0.0f, 300.0f, 0.5f);
            Expect.True(unit.IsPushed(), "it is pushed");
            Within(1200.0f, unit.PushVelocity().X, 0.01f, "at 2 * 300 / 0.5 along the angle");
            Within(0.0f, unit.PushVelocity().Y, 0.01f, "and none across it");
            Expect.True(Tracking.Running, "the cycle's timer runs for a pushed unit with nothing tracked");
            Steps(world, 15);
            Expect.True(unit.IsPushed(), "still pushed before 0.5 s");
            Steps(world, 2);
            Expect.True(!unit.IsPushed(), "its push has ended within a step of 0.5 s");
            Within(812.0f, unit.X, 4.0f, "300 away");
            Within(512.0f, unit.Y, 0.01f, "along the angle");
            Within(0.0f, unit.PushVelocity().X, 0.0f, "no push left");
            Expect.Equal(0, unit.Slams.Count, "no OnSlammed");
            Expect.Equal(1, unit.Pushers.Count, "OnPushed once");
            Expect.True(unit.Pushers[0] == null, "with no pusher");
            Expect.Same(unit, unit.RanWith, "the reactions run with the unit as the current owner");
            Expect.True(!Tracking.Running, "the cycle's timer stops once no unit is pushed");
            var x = unit.X;
            Steps(world, 4);
            Within(x, unit.X, 0.0f, "it stays where it stopped");
            var turned = PusheeAt(world, 512.0f, 1024.0f);
            turned.Knockback(90.0f, 300.0f, 0.5f);
            Steps(world, 17);
            Within(512.0f, turned.X, 0.01f, "an angle of 90 degrees: x kept");
            Within(1324.0f, turned.Y, 4.0f, "and 300 up");
        }

        /// <summary>Pushed square into an unwalkable line: it stops short of it, and OnSlammed runs once, with the speed it hit at, the normal out of the line and the unit's place; its pusher, removed meanwhile, reads null.</summary>
        public static void Blocked(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            FakeInteractionWorld interactions;
            Fresh(out terrain, out interactions);
            LineAt(terrain, 650.0f, FakeTerrainWorld.NoWalk);
            var unit = PusheeAt(world, 512.0f, 512.0f);
            var pusher = FootmanAt(world, 400.0f, 512.0f);
            Pushes.Push(unit, 600.0f, 0.0f, 0.0f, pusher);
            Expect.Same(pusher, unit.Pushers[0], "OnPushed is given the pusher");
            pusher.Remove();
            var before = 0.0f;
            for (var i = 0; i < 64 && unit.Slams.Count == 0; i++)
            {
                before = Speed(unit);
                Steps(world, 1);
            }

            Expect.Equal(1, unit.Slams.Count, "OnSlammed once");
            Within(before, unit.SlamSpeeds[0], 0.01f, "with the speed it hit at");
            Expect.True(unit.SlamPushers[0] == null, "a pusher that has ended reads null");
            var slam = unit.Slams[0];
            Within(-1.0f, slam.Normal.X, 0.001f, "the normal out of the line: x");
            Within(0.0f, slam.Normal.Y, 0.001f, "the normal: y");
            Within(0.0f, slam.Normal.Z, 0.001f, "the normal: z");
            Within(unit.X, slam.X, 0.001f, "at the unit's place: x");
            Within(512.0f, slam.Y, 0.001f, "at the unit's place: y");
            Expect.True(!unit.IsPushed(), "it has stopped");
            Expect.True(unit.X + 16.0f < 640.0f, "short of the line, its collision size off; at " + unit.X);
            Expect.True(unit.X > 560.0f, "after moving some way; at " + unit.X);
            var x = unit.X;
            Steps(world, 16);
            Within(x, unit.X, 0.0f, "it stays");
            Expect.Equal(1, unit.Slams.Count, "and is not slammed again");
        }

        /// <summary>Pushed at 45 degrees into an unwalkable line: it loses the speed into the line (OnSlammed once with it) and slides on along it.</summary>
        public static void SlideAlong(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            FakeInteractionWorld interactions;
            Fresh(out terrain, out interactions);
            LineAt(terrain, 650.0f, FakeTerrainWorld.NoWalk);
            var unit = PusheeAt(world, 560.0f, 512.0f);
            unit.Push(400.0f, 400.0f);
            var into = 0.0f;
            for (var i = 0; i < 64 && unit.Slams.Count == 0; i++)
            {
                into = unit.PushVelocity().X;
                Steps(world, 1);
            }

            Expect.Equal(1, unit.Slams.Count, "OnSlammed once");
            Within(into, unit.SlamSpeeds[0], 0.01f, "with the speed into the line");
            Within(-1.0f, unit.Slams[0].Normal.X, 0.001f, "the normal out of the line");
            Expect.True(unit.IsPushed(), "still pushed");
            Within(0.0f, unit.PushVelocity().X, 0.0f, "with no speed into the line");
            Expect.True(unit.PushVelocity().Y > 300.0f, "and its speed along it; was " + unit.PushVelocity().Y);
            var x = unit.X;
            var y = unit.Y;
            Steps(world, 4);
            Expect.True(unit.Y > y + 30.0f, "it keeps moving along the line; from " + y + " to " + unit.Y);
            Within(x, unit.X, 0.0f, "not into it");
            Expect.Equal(1, unit.Slams.Count, "slammed once only");
        }

        /// <summary>Pushed into a tree, where the ground is walkable but the item trick lands away: it stops short, with OnSlammed once; its pusher, still in the game, is the slam's.</summary>
        public static void Tree(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            FakeInteractionWorld interactions;
            Fresh(out terrain, out interactions);
            terrain.Occupied = (x, y) => (x - 700.0f) * (x - 700.0f) + (y - 512.0f) * (y - 512.0f) < 32.0f * 32.0f;
            var pusher = FootmanAt(world, 400.0f, 512.0f);
            var unit = PusheeAt(world, 600.0f, 512.0f);
            Pushes.Push(unit, 600.0f, 0.0f, 0.0f, pusher);
            Steps(world, 32);
            Expect.Equal(1, unit.Slams.Count, "OnSlammed once");
            Expect.Same(pusher, unit.SlamPushers[0], "the pusher, still in the game");
            Expect.True(!unit.IsPushed(), "it has stopped");
            Expect.True(unit.X + 16.0f < 668.0f, "short of the tree; at " + unit.X);
            Expect.True(Terrain.IsWalkable(690.0f, 512.0f), "where the ground itself is walkable");
        }

        /// <summary>Push(0, 0, 400) on a footman: its fly height rises to 100 and falls back to 0 under gravity, OnGrounded runs once at 400, and the push ends; it does not move along the ground.</summary>
        public static void LiftLand(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            FakeInteractionWorld interactions;
            Fresh(out terrain, out interactions);
            var unit = PusheeAt(world, 512.0f, 512.0f);
            unit.Push(0.0f, 0.0f, 400.0f);
            Within(400.0f, unit.PushVelocity().Z, 0.01f, "it is pushed up");
            var peak = 0.0f;
            var steps = 0;
            for (var i = 0; i < 64 && unit.IsPushed(); i++)
            {
                Steps(world, 1);
                steps++;
                if (unit.FlyHeight > peak)
                {
                    peak = unit.FlyHeight;
                }
            }

            Within(100.0f, peak, 1.0f, "it rises to 400 * 400 / (2 * 800)");
            Expect.True(steps >= 32 && steps <= 33, "it is down after a second, within a step; " + steps + " steps");
            Expect.Equal(1, unit.Landings.Count, "OnGrounded once");
            Within(400.0f, unit.Landings[0], 1.0f, "at the speed it left at");
            Within(0.0f, unit.FlyHeight, 0.0f, "on the ground");
            Expect.True(!unit.IsPushed(), "the push has ended");
            Within(512.0f, unit.X, 0.0f, "x kept");
            Within(512.0f, unit.Y, 0.0f, "y kept");
            Expect.Equal(0, unit.Slams.Count, "no OnSlammed");
        }

        /// <summary>A flyer pushed up drifts back to its type's height and stops there, never landing; along the ground it passes over what is not walkable, and is stopped by what is not flyable.</summary>
        public static void Flyer(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            FakeInteractionWorld interactions;
            var fake = Fresh(out terrain, out interactions);
            var handle = world.NewUnit(ObjectTypes.FourCC(PusheeId));
            world.MoveUnit(handle, 512.0f, 512.0f);
            world.MakeFlying(handle);
            world.SetFlyHeight(handle, 240.0f);
            fake.Flyers.Add(handle);
            fake.DefaultHeights[handle] = 240.0f;
            var unit = (Pushee)Unit.Of(handle)!;
            unit.Push(0.0f, 0.0f, 400.0f);
            var highest = 0.0f;
            var lowest = 1000.0f;
            for (var i = 0; i < 96 && unit.IsPushed(); i++)
            {
                Steps(world, 1);
                highest = Math.Max(highest, unit.FlyHeight);
                lowest = Math.Min(lowest, unit.FlyHeight);
            }

            Expect.True(highest > 300.0f, "it rose; to " + highest);
            Expect.True(lowest >= 239.99f, "it never went below its height; to " + lowest);
            Expect.True(!unit.IsPushed(), "the push has ended");
            Within(240.0f, unit.FlyHeight, 0.01f, "back at its type's height");
            Expect.Equal(0, unit.Landings.Count, "no OnGrounded");
            LineAt(terrain, 650.0f, FakeTerrainWorld.NoWalk);
            LineAt(terrain, 1000.0f, FakeTerrainWorld.NoFly);
            unit.Push(1200.0f, 0.0f);
            Steps(world, 64);
            Expect.True(unit.X > 700.0f, "over the unwalkable line; at " + unit.X);
            Expect.True(unit.X + 16.0f < 992.0f, "stopped short of the unflyable one; at " + unit.X);
            Expect.Equal(1, unit.Slams.Count, "slammed once, by the unflyable line");
        }

        /// <summary>A second knockback adds its velocity, and the slowing is the latest knockback's until its time is over, then Body.Friction (600 without a Body); a Push adds its velocity and keeps the slowing.</summary>
        public static void Overlap(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            FakeInteractionWorld interactions;
            Fresh(out terrain, out interactions);
            var unit = PusheeAt(world, 100.0f, 100.0f);
            unit.Knockback(0.0f, 300.0f, 0.5f);
            Steps(world, 2);
            Within(1050.0f, Speed(unit), 0.01f, "1,200 less 2,400 a second for 2 steps");
            unit.Knockback(90.0f, 100.0f, 1.0f);
            Within(1050.0f, unit.PushVelocity().X, 0.01f, "the first's velocity kept");
            Within(200.0f, unit.PushVelocity().Y, 0.01f, "the second's added");
            Expect.Equal(2, unit.Pushers.Count, "OnPushed at each");
            var before = Speed(unit);
            Steps(world, 1);
            Within(before - 200.0f / 32.0f, Speed(unit), 0.01f, "the slowing is the latest's, 200 a second");
            Steps(world, 32);
            before = Speed(unit);
            Steps(world, 1);
            Within(before - 600.0f / 32.0f, Speed(unit), 0.01f, "Body.Friction once the knockback's time is over");
            var along = unit.PushVelocity().Y;
            unit.Push(0.0f, 100.0f);
            Within(along + 100.0f, unit.PushVelocity().Y, 0.01f, "a Push adds its velocity");
            before = Speed(unit);
            Steps(world, 1);
            Within(before - 600.0f / 32.0f, Speed(unit), 0.01f, "and keeps the slowing");
            Expect.Equal(3, unit.Pushers.Count, "OnPushed at each push");
        }

        /// <summary>A pushed unit that dies, is removed, is hidden or is loaded: its push ends, it is moved no more and no reaction runs, though each was about to slam into a line; a lifted one that dies is put down with no OnGrounded.</summary>
        public static void EndsOnDeath(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            FakeInteractionWorld interactions;
            Fresh(out terrain, out interactions);
            LineAt(terrain, 650.0f, FakeTerrainWorld.NoWalk);
            var dead = PusheeAt(world, 600.0f, 300.0f);
            var gone = PusheeAt(world, 600.0f, 500.0f);
            var hidden = PusheeAt(world, 600.0f, 700.0f);
            var loaded = PusheeAt(world, 600.0f, 900.0f);
            var lifted = PusheeAt(world, 300.0f, 1100.0f);
            dead.Push(600.0f, 0.0f);
            gone.Push(600.0f, 0.0f);
            hidden.Push(600.0f, 0.0f);
            loaded.Push(600.0f, 0.0f);
            lifted.Push(0.0f, 0.0f, 400.0f);
            Steps(world, 1);
            Expect.True(lifted.FlyHeight > 0.0f, "the lifted one is up");
            dead.Kill();
            gone.Remove();
            interactions.Hidden.Add(hidden.Handle);
            interactions.Loaded.Add(loaded.Handle);
            lifted.Kill();
            var places = Places(world);
            TestLog.Lines.Clear();
            Steps(world, 64);
            Expect.Equal(places, Places(world), "none is moved again");
            Expect.Equal(0, TestLog.Lines.Count, "no reaction runs");
            Expect.True(!dead.IsPushed() && !gone.IsPushed() && !hidden.IsPushed() && !loaded.IsPushed() && !lifted.IsPushed(), "every push has ended");
            Expect.Equal(0, dead.Slams.Count + gone.Slams.Count + hidden.Slams.Count + loaded.Slams.Count, "no OnSlammed");
            Expect.Equal(0, lifted.Landings.Count, "no OnGrounded");
            Within(0.0f, lifted.FlyHeight, 0.0f, "the lifted corpse is put down");
            Expect.True(!Tracking.Running, "the cycle's timer stops");
        }

        /// <summary>A structure is never pushed, nor a unit whose Body is immovable: no move, no lift, no OnPushed.</summary>
        public static void Structure(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            FakeInteractionWorld interactions;
            var fake = Fresh(out terrain, out interactions);
            var building = PusheeAt(world, 512.0f, 512.0f);
            fake.Structures.Add(building.Handle);
            building.Knockback(0.0f, 300.0f, 0.5f);
            building.Push(100.0f, 0.0f, 100.0f);
            Expect.True(!building.IsPushed(), "a structure is not pushed");
            var still = PusheeAt(world, 512.0f, 1024.0f);
            still.Add(new Body { Immovable = true });
            still.Knockback(0.0f, 300.0f, 0.5f);
            Expect.True(!still.IsPushed(), "nor an immovable Body");
            Steps(world, 32);
            Expect.Equal(0, Places(world), "neither moved");
            Within(512.0f, building.X, 0.0f, "the structure stays");
            Within(0.0f, building.FlyHeight, 0.0f, "on the ground");
            Within(512.0f, still.X, 0.0f, "the immovable unit stays");
            Expect.Equal(0, building.Pushers.Count + still.Pushers.Count, "no OnPushed");
            Within(0.0f, building.PushVelocity().X, 0.0f, "no push velocity");
        }

        /// <summary>Body: unset, its mass is the unit's collision size, its height 4 times it, its friction 600 and it is movable; a structure's is immovable. Set values are kept, the unit's height follows Body.Height, and a push slows at Body.Friction.</summary>
        public static void BodyDefaults(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            FakeInteractionWorld interactions;
            var fake = Fresh(out terrain, out interactions);
            var loose = new Body();
            Within(0.0f, loose.Mass, 0.0f, "a Body on no unit has no mass yet");
            var unit = FootmanAt(world, 512.0f, 512.0f);
            fake.Sizes[unit.Handle] = 24.0f;
            Within(96.0f, Shapes.UnitHeight(unit), 0.0f, "with no Body a unit is 4 times its collision size tall");
            var body = unit.Add(new Body());
            Within(24.0f, body.Mass, 0.0f, "Mass: the collision size");
            Within(96.0f, body.Height, 0.0f, "Height: 4 times it");
            Within(600.0f, body.Friction, 0.0f, "Friction: 600");
            Expect.True(!body.Immovable, "movable");
            body.Mass = 40.0f;
            body.Height = 50.0f;
            body.Friction = 300.0f;
            Within(40.0f, body.Mass, 0.0f, "Mass set");
            Within(50.0f, body.Height, 0.0f, "Height set");
            Within(50.0f, Shapes.UnitHeight(unit), 0.0f, "the unit's height is its Body's");
            unit.Push(300.0f, 0.0f);
            Steps(world, 1);
            Within(300.0f - 300.0f / 32.0f, Speed(unit), 0.01f, "it slows at its Body's friction");
            fake.Structures.Add(unit.Handle);
            Expect.True(body.Immovable, "a structure's Body is immovable");
        }

        /// <summary>A push moves the unit with SetUnitX/Y only: the fake records no order given or cleared and no move through pathing.</summary>
        public static void OrderKept(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            FakeInteractionWorld interactions;
            Fresh(out terrain, out interactions);
            var unit = FootmanAt(world, 512.0f, 512.0f);
            unit.OrderAt("move", 512.0f, 1500.0f);
            world.UnitCalls.Clear();
            world.Orders.Clear();
            unit.Knockback(0.0f, 200.0f, 0.5f);
            Steps(world, 20);
            Expect.Equal(0, world.Orders.Count, "no order given or cleared");
            Expect.True(Places(world) >= 16, "it was put at its places; " + Places(world) + " times");
            Expect.Equal(Places(world), world.UnitCalls.Count, "with SetUnitX/Y alone");
        }

        /// <summary>A unit pushed 600 in a step is found by Find.InRange at its new place in the gizmos' pass of the same step, before the grid would read it again.</summary>
        public static void Cell(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            FakeInteractionWorld interactions;
            Fresh(out terrain, out interactions);
            var unit = FootmanAt(world, 100.0f, 100.0f);
            Expect.Equal(1, Find.InRange<Unit>(100.0f, 100.0f, 10.0f).Count, "found where it stands");
            var ticks = 0;
            var missed = 0;
            var seeker = Gizmo.Create(new PushSeeker(), 1500.0f, 1500.0f);
            seeker.Ticked = () =>
            {
                if (unit.X > 300.0f)
                {
                    ticks++;
                    if (Find.InRange<Unit>(unit.X, unit.Y, 10.0f).Count != 1)
                    {
                        missed++;
                    }
                }
            };
            unit.Push(19200.0f, 0.0f);
            Steps(world, 2);
            unit.StopPush();
            Expect.True(ticks > 0, "a tick saw it moved");
            Expect.Equal(0, missed, "it is found at its new place in the step it moved");
            Expect.True(unit.X > 1000.0f, "it moved about 600 a step; at " + unit.X);
        }

        /// <summary>Knockback with a distance of 0 does nothing; with a time of 0 or less it fails in a development build and does nothing in a release build. Push with no velocity does nothing. Nothing becomes NaN.</summary>
        public static void Zero(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            FakeInteractionWorld interactions;
            Fresh(out terrain, out interactions);
            var unit = PusheeAt(world, 512.0f, 512.0f);
            unit.Knockback(45.0f, 0.0f, 0.5f);
            Expect.True(!unit.IsPushed(), "a distance of 0: no push");
            Expect.Equal(0, unit.Pushers.Count, "and no OnPushed");
            unit.Push(0.0f, 0.0f, 0.0f);
            Expect.True(!unit.IsPushed(), "a push of nothing: no push");
#if DEBUG
            Expect.Throws<MoonwellException>(() => unit.Knockback(0.0f, 100.0f, 0.0f), "a time of 0 fails in a development build");
            Expect.Throws<MoonwellException>(() => unit.Knockback(0.0f, 100.0f, -1.0f), "and a time below 0");
#else
            unit.Knockback(0.0f, 100.0f, 0.0f);
#endif
            Expect.True(!unit.IsPushed(), "no push");
            Steps(world, 8);
            Within(512.0f, unit.X, 0.0f, "it stays");
            var velocity = unit.PushVelocity();
            Expect.True(!float.IsNaN(velocity.X) && !float.IsNaN(velocity.Y) && !float.IsNaN(velocity.Z), "no NaN");
            Expect.True(!float.IsNaN(unit.X) && !float.IsNaN(unit.Y), "no NaN place");
        }

        /// <summary>The reactions run on the unit's class, then its components and buffs in the order they were added, each guarded: a failing one is reported and the rest run, and a method a class does not write is not called.</summary>
        public static void ReactionOrder(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            FakeInteractionWorld interactions;
            Fresh(out terrain, out interactions);
            LineAt(terrain, 650.0f, FakeTerrainWorld.NoWalk);
            var unit = PusheeAt(world, 600.0f, 512.0f);
            unit.Add(new PushThrower());
            unit.Apply(new PushGuard());
            TestLog.Lines.Clear();
            unit.Push(600.0f, 0.0f);
            Expect.Equal(4, TestLog.Lines.Count, "four OnPushed");
            Expect.Equal("unit pushed", TestLog.Lines[0], "the class first");
            Expect.Equal("component pushed", TestLog.Lines[1], "then the component from its constructor");
            Expect.Equal("thrower pushed", TestLog.Lines[2], "then the one added after");
            Expect.Equal("buff pushed", TestLog.Lines[3], "then the buff, after the failing one");
            Expect.Equal(1, world.Logged.Count, "the failure is reported");
            Expect.Equal("Moonwell: PushThrower.OnPushed failed: thrown on purpose", world.Logged[0], "as the reaction's");
            TestLog.Lines.Clear();
            Steps(world, 8);
            Expect.Equal(3, TestLog.Lines.Count, "three OnSlammed: the thrower writes none");
            Expect.Equal("unit slammed", TestLog.Lines[0], "the class first");
            Expect.Equal("component slammed", TestLog.Lines[1], "then the component");
            Expect.Equal("buff slammed", TestLog.Lines[2], "then the buff");
            unit.Push(0.0f, 0.0f, 200.0f);
            TestLog.Lines.Clear();
            Steps(world, 32);
            Expect.Equal(2, TestLog.Lines.Count, "two OnGrounded: the components write none");
            Expect.Equal("unit grounded", TestLog.Lines[0], "the class first");
            Expect.Equal("buff grounded", TestLog.Lines[1], "then the buff");
            Expect.Equal(1, world.Logged.Count, "no other failure: nothing unwritten was called");
        }

        /// <summary>Pushed at 45 degrees into an unwalkable row across y: it loses the speed into the row (OnSlammed once with it, the normal down) and slides on along x.</summary>
        public static void SlideAlongX(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            FakeInteractionWorld interactions;
            Fresh(out terrain, out interactions);
            RowAt(terrain, 650.0f, FakeTerrainWorld.NoWalk);
            var unit = PusheeAt(world, 512.0f, 560.0f);
            unit.Push(400.0f, 400.0f);
            var into = 0.0f;
            for (var i = 0; i < 64 && unit.Slams.Count == 0; i++)
            {
                into = unit.PushVelocity().Y;
                Steps(world, 1);
            }

            Expect.Equal(1, unit.Slams.Count, "OnSlammed once");
            Within(into, unit.SlamSpeeds[0], 0.01f, "with the speed into the row");
            Within(0.0f, unit.Slams[0].Normal.X, 0.001f, "the normal out of the row: x");
            Within(-1.0f, unit.Slams[0].Normal.Y, 0.001f, "the normal out of the row: y");
            Expect.True(unit.IsPushed(), "still pushed");
            Within(0.0f, unit.PushVelocity().Y, 0.0f, "with no speed into the row");
            Expect.True(unit.PushVelocity().X > 300.0f, "and its speed along it; was " + unit.PushVelocity().X);
            var x = unit.X;
            var y = unit.Y;
            Steps(world, 4);
            Expect.True(unit.X > x + 30.0f, "it keeps moving along the row; from " + x + " to " + unit.X);
            Within(y, unit.Y, 0.0f, "not into it");
            Expect.Equal(1, unit.Slams.Count, "slammed once only");
        }

        /// <summary>In one pass, one unit's OnSlammed kills it and the next unit's OnSlammed pushes it back: the pass goes on to the second, nothing runs on the dead one after its own reaction (not its component's), the second bounces away, and once every push has ended the cycle's Awake count is back to 0.</summary>
        public static void Reentrant(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            FakeInteractionWorld interactions;
            Fresh(out terrain, out interactions);
            LineAt(terrain, 650.0f, FakeTerrainWorld.NoWalk);
            var suicide = PusheeAt(world, 600.0f, 300.0f);
            var bouncer = PusheeAt(world, 600.0f, 700.0f);
            suicide.Slamming = slam => suicide.Kill();
            bouncer.Slamming = slam => bouncer.Push(-400.0f, 0.0f);
            suicide.Push(600.0f, 0.0f);
            bouncer.Push(600.0f, 0.0f);
            TestLog.Lines.Clear();
            for (var i = 0; i < 16 && bouncer.Slams.Count == 0; i++)
            {
                Steps(world, 1);
            }

            Expect.Equal(1, suicide.Slams.Count, "the first slammed");
            Expect.Equal(1, bouncer.Slams.Count, "the second slammed in the same pass");
            var components = 0;
            for (var i = 0; i < TestLog.Lines.Count; i++)
            {
                if (TestLog.Lines[i] == "component slammed")
                {
                    components++;
                }
            }

            Expect.Equal(1, components, "only the living one's component ran");
            Expect.True(!suicide.IsPushed(), "the dead one's push has ended");
            Expect.True(bouncer.IsPushed(), "the second is pushed again");
            Expect.True(bouncer.PushVelocity().X < 0.0f, "back from the line");
            var deadX = suicide.X;
            var x = bouncer.X;
            Steps(world, 4);
            Expect.True(bouncer.X < x - 20.0f, "it bounces away; from " + x + " to " + bouncer.X);
            Within(deadX, suicide.X, 0.0f, "the dead one is not moved");
            Steps(world, 60);
            Expect.True(!bouncer.IsPushed(), "the bounce has ended");
            Expect.Equal(1, suicide.Slams.Count + suicide.Landings.Count, "no reaction on the dead one after");
            Expect.Equal(1, bouncer.Slams.Count, "the bouncer slammed once");
            Expect.Equal(0, Tracking.Awake, "the Awake count is back to 0");
            Expect.True(!Tracking.Running, "and the cycle's timer stops");
            Expect.Equal(0, world.Logged.Count, "no failure");
        }

        /// <summary>A unit that does not fly whose type hovers at 50: lifted, it comes back down to 50, not to the ground, with OnGrounded once.</summary>
        public static void Hover(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            FakeInteractionWorld interactions;
            var fake = Fresh(out terrain, out interactions);
            var unit = PusheeAt(world, 512.0f, 512.0f);
            fake.DefaultHeights[unit.Handle] = 50.0f;
            unit.FlyHeight = 50.0f;
            unit.Push(0.0f, 0.0f, 400.0f);
            var lowest = 1000.0f;
            var highest = 0.0f;
            for (var i = 0; i < 64 && unit.IsPushed(); i++)
            {
                Steps(world, 1);
                lowest = Math.Min(lowest, unit.FlyHeight);
                highest = Math.Max(highest, unit.FlyHeight);
            }

            Within(150.0f, highest, 1.0f, "it rises 100 above its height");
            Expect.True(lowest >= 50.0f, "never below its height; to " + lowest);
            Expect.True(!unit.IsPushed(), "the push has ended");
            Within(50.0f, unit.FlyHeight, 0.0f, "back at its type's height");
            Expect.Equal(1, unit.Landings.Count, "OnGrounded once");
            Within(400.0f, unit.Landings[0], 1.0f, "at the speed it left at");
        }

        /// <summary>A unit class that writes both IDamageSource.OnLanded(Hit) and IPushTarget.OnGrounded(float): the damage package and the push both work out its reactions without error, and coming down after a lift runs OnGrounded, never OnLanded.</summary>
        public static void GroundedNames(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            FakeInteractionWorld interactions;
            Fresh(out terrain, out interactions);
            DamageSystem.Install(new FakeDamageWorld(world));
            ObjectTypes.Unit(ObjectTypes.FourCC("h0P2"), () => new GroundedSource());
            var handle = world.NewUnit(ObjectTypes.FourCC("h0P2"));
            world.MoveUnit(handle, 512.0f, 512.0f);
            var unit = (GroundedSource)Unit.Of(handle)!;
            Moonwell.Reactions.Learn(unit);
            Expect.True(Moonwell.Reactions.Wants(Stage.Landed), "the damage package sees its OnLanded");
            TestLog.Lines.Clear();
            unit.Push(0.0f, 0.0f, 400.0f);
            Steps(world, 40);
            Expect.True(!unit.IsPushed(), "the push has ended");
            Expect.Equal(1, TestLog.Lines.Count, "one reaction");
            Expect.Equal("grounded", TestLog.Lines[0], "OnGrounded, not OnLanded");
            Within(400.0f, unit.Groundings[0], 1.0f, "with its speed");
            Expect.Equal(0, world.Logged.Count, "no failure");
        }
    }
}
