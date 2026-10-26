using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>Cases for a missile's motion, its launch from a unit, its ending and its look, with the default step of 1/32 second on flat ground at 0 unless a case says otherwise.</summary>
    public static class FlightCases
    {
        /// <summary>Pi, in radians.</summary>
        private const float Pi = 3.14159265f;

        /// <summary>The pitch of a velocity of (-300, 0, 400), climbing: minus the angle whose tangent is 4/3.</summary>
        private const float Climbing = -0.92729522f;

        /// <summary>Installs the interactions package and the physics package on fake worlds, and flat ground at 0; gives the physics fake.</summary>
        private static FakePhysicsWorld Fresh()
        {
            Interactions.Install(new FakeInteractionWorld());
            var fake = new FakePhysicsWorld();
            Physics.Install(fake);
            var flat = GridText.Corner(0.0f, 2);
            Terrain.Install(new FakeTerrainWorld(), 2, 2, 0.0f, 0.0f, GridText.Tile(flat, flat, flat, flat));
            return fake;
        }

        /// <summary>Installs a hill along x: two rows of corners, 0 and 128 up, every 128 along x from 0 at the heights 0, 0, 50, 150, 200, 150, 50, 0, 0, so the ground is the same at every y.</summary>
        private static void InstallHill()
        {
            var hill = new float[] { 0.0f, 0.0f, 50.0f, 150.0f, 200.0f, 150.0f, 50.0f, 0.0f, 0.0f };
            var row = "";
            for (var i = 0; i < hill.Length; i++)
            {
                row = row + GridText.Corner(hill[i], 2);
            }

            Terrain.Install(new FakeTerrainWorld(), hill.Length, 2, 0.0f, 0.0f, row + row);
        }

        /// <summary>Moves the game on by a number of steps, one step at a time.</summary>
        private static void Steps(FakeWorld world, int count)
        {
            for (var i = 0; i < count; i++)
            {
                world.Advance(Interactions.Step);
            }
        }

        /// <summary>Makes a footman of the game at a place with a facing, and takes it in.</summary>
        private static Unit UnitAt(FakeWorld world, float x, float y, float facing)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            world.MoveUnit(handle, x, y);
            world.FaceUnitNow(handle, facing);
            return Unit.Of(handle)!;
        }

        /// <summary>Fails unless two floats are within a tolerance.</summary>
        private static void Within(float expected, float actual, float tolerance, string what)
        {
            Expect.True(Math.Abs(expected - actual) <= tolerance, what + ": expected " + expected + " within " + tolerance + ", was " + actual);
        }

        /// <summary>Fails when a float is not a number.</summary>
        private static void Number(float value, string what)
        {
            Expect.True(!float.IsNaN(value), what + " is a number");
        }

        /// <summary>A missile with a speed of 900 and no direction goes along x, and is at 900 after 32 steps; Speed keeps the direction it is given, and Velocity reads the floats.</summary>
        public static void Straight(FakeWorld world)
        {
            Fresh();
            var dart = new Dart();
            dart.Speed = 900.0f;
            Within(900.0f, dart.VX, 0.001f, "a speed on a still missile goes along x");
            Within(0.0f, dart.VY, 0.001f, "no y");
            Missile.Create(dart, 0.0f, 0.0f, 100.0f);
            Within(100.0f, dart.Z, 0.001f, "Create puts it at its height");
            Steps(world, 32);
            Within(900.0f, dart.X, 0.01f, "x after 32 steps");
            Within(0.0f, dart.Y, 0.01f, "y after 32 steps");
            Within(100.0f, dart.Z, 0.01f, "z after 32 steps");
            dart.Velocity = new Vector3(3.0f, 4.0f, 0.0f);
            dart.Speed = 10.0f;
            Within(6.0f, dart.VX, 0.001f, "Speed keeps the direction: x");
            Within(8.0f, dart.VY, 0.001f, "Speed keeps the direction: y");
            Within(0.0f, dart.VZ, 0.001f, "Speed keeps the direction: z");
            Within(10.0f, dart.Speed, 0.001f, "the speed read");
            var velocity = dart.Velocity;
            Within(6.0f, velocity.X, 0.001f, "Velocity.X");
            Within(8.0f, velocity.Y, 0.001f, "Velocity.Y");
            Expect.True(!dart.HasEnded, "it flies on");
        }

        /// <summary>With Gravity 800, launched up at 400 from 0, it peaks at 100 (v squared over 2g) and is back at 0 after 1 second (2v/g), 32 steps.</summary>
        public static void Gravity(FakeWorld world)
        {
            Fresh();
            var dart = new Dart();
            dart.Gravity = 800.0f;
            dart.VZ = 400.0f;
            dart.VX = 100.0f;
            Missile.Create(dart, 0.0f, 0.0f, 0.0f);
            var peak = -1000.0f;
            var landed = -1;
            for (var step = 1; step <= 40; step++)
            {
                Steps(world, 1);
                if (dart.Z > peak)
                {
                    peak = dart.Z;
                }

                if (landed < 0 && step > 1 && dart.Z <= 0.01f)
                {
                    landed = step;
                }
            }

            Within(100.0f, peak, 0.5f, "the peak");
            Expect.True(landed >= 31 && landed <= 33, "back at 0 after 32 steps, within one; was " + landed);
            Within(125.0f, dart.X, 0.01f, "x went on at 100 a second for 40 steps");
        }

        /// <summary>From 300 a second with Acceleration 300 and MaxSpeed 600: 450 after half a second, 600 after a second, and no more after.</summary>
        public static void Acceleration(FakeWorld world)
        {
            Fresh();
            var dart = new Dart();
            dart.Speed = 300.0f;
            dart.Acceleration = 300.0f;
            dart.MaxSpeed = 600.0f;
            Missile.Create(dart, 0.0f, 0.0f, 100.0f);
            Steps(world, 16);
            Within(450.0f, dart.Speed, 0.01f, "the speed after half a second");
            Steps(world, 16);
            Within(600.0f, dart.Speed, 0.01f, "the speed after a second");
            Steps(world, 32);
            Within(600.0f, dart.Speed, 0.01f, "the speed a second later");
            Within(0.0f, dart.VY, 0.001f, "along x still");
        }

        /// <summary>Fired at a point 800 away with Arc 0.35 (and no launch height, so the chord is level): it peaks at 280 and lands within 2 of the point at the ground's height.</summary>
        public static void Arc(FakeWorld world)
        {
            Fresh();
            var source = UnitAt(world, 0.0f, 0.0f, 0.0f);
            var dart = new Dart();
            dart.Speed = 700.0f;
            dart.Arc = 0.35f;
            dart.LaunchHeight = 0.0f;
            source.Fire(dart, 800.0f, 0.0f);
            Expect.Same(source, dart.Source, "the source");
            Expect.Same(source.Player, dart.Player, "the source's player");
            Within(0.0f, dart.Z, 0.01f, "it starts on the ground");
            Expect.True(dart.Gravity > 0.0f, "Fire gave it a gravity");
            var peak = -1000.0f;
            var landX = float.NaN;
            var landY = float.NaN;
            var previousX = dart.X;
            var previousY = dart.Y;
            var previousZ = dart.Z;
            for (var step = 1; step <= 64; step++)
            {
                Steps(world, 1);
                if (dart.Z > peak)
                {
                    peak = dart.Z;
                }

                if (step > 1 && dart.Z <= 0.0f && previousZ > 0.0f)
                {
                    var share = previousZ / (previousZ - dart.Z);
                    landX = previousX + (dart.X - previousX) * share;
                    landY = previousY + (dart.Y - previousY) * share;
                    break;
                }

                previousX = dart.X;
                previousY = dart.Y;
                previousZ = dart.Z;
            }

            Within(280.0f, peak, 2.0f, "the peak");
            Within(800.0f, landX, 2.0f, "where it lands: x");
            Within(0.0f, landY, 2.0f, "where it lands: y");
        }

        /// <summary>Fired with no arc at a unit, it aims at the unit's middle, half its height of 4 times its collision size above the ground plus its fly height, and flies on past it.</summary>
        public static void AimUnit(FakeWorld world)
        {
            var fake = Fresh();
            var source = UnitAt(world, 0.0f, 0.0f, 0.0f);
            var target = UnitAt(world, 500.0f, 0.0f, 180.0f);
            fake.Sizes[target.Handle] = 8.0f;
            var dart = new Dart();
            dart.Speed = 1000.0f;
            source.Fire(dart, target);
            Within(60.0f, dart.Z, 0.01f, "it leaves at the launch height");
            Within(1000.0f, dart.Speed, 0.01f, "at its speed");
            Within((16.0f - 60.0f) / 500.0f, dart.VZ / dart.VX, 0.001f, "it aims at 16 up, the middle of a unit 32 tall");
            Steps(world, 16);
            Within(490.0f, dart.X, 10.0f, "near the unit after 16 steps");
            Within(60.0f + dart.X * (16.0f - 60.0f) / 500.0f, dart.Z, 0.01f, "on the line through the unit's middle");
            Steps(world, 32);
            Expect.True(!dart.HasEnded && dart.X > 1400.0f, "it flies on past");
            Within(60.0f + dart.X * (16.0f - 60.0f) / 500.0f, dart.Z, 0.01f, "on the same line past it");
            var flyer = UnitAt(world, 0.0f, 400.0f, 0.0f);
            world.SetFlyHeight(flyer.Handle, 100.0f);
            var second = new Dart();
            second.Speed = 1000.0f;
            source.Fire(second, flyer);
            var length = MathF.Sqrt(400.0f * 400.0f + (100.0f + 32.0f - 60.0f) * (100.0f + 32.0f - 60.0f));
            Within(1000.0f * (100.0f + 32.0f - 60.0f) / length, second.VZ, 0.05f, "a flyer's middle is its fly height and half its height up");
            Within(1000.0f * 400.0f / length, second.VY, 0.05f, "toward the flyer");
        }

        /// <summary>Fired with no arc at a point on flat ground, it leaves its launch offset ahead and flies level at the launch height, on past the point; on a hill it aims at the launch height above the ground at the point.</summary>
        public static void AimPointLevel(FakeWorld world)
        {
            Fresh();
            var source = UnitAt(world, 0.0f, 0.0f, 0.0f);
            var dart = new Dart();
            dart.Speed = 900.0f;
            dart.LaunchOffset = 50.0f;
            source.Fire(dart, 1000.0f, 0.0f);
            Within(50.0f, dart.X, 0.01f, "it leaves its offset ahead");
            Within(60.0f, dart.Z, 0.01f, "at the launch height");
            Within(0.0f, dart.VZ, 0.001f, "level");
            Within(900.0f, dart.VX, 0.01f, "along x at its speed");
            Steps(world, 64);
            Within(1850.0f, dart.X, 0.05f, "on past the point");
            Within(60.0f, dart.Z, 0.01f, "still level");
            Expect.True(!dart.HasEnded, "it has not ended");
            InstallHill();
            var climber = UnitAt(world, 0.0f, 64.0f, 0.0f);
            var up = new Dart();
            up.Speed = 900.0f;
            climber.Fire(up, 512.0f, 64.0f);
            Within(60.0f, up.Z, 0.01f, "it leaves 60 above the foot of the hill");
            Within((260.0f - 60.0f) / 512.0f, up.VZ / up.VX, 0.001f, "it aims 60 above the hilltop");
        }

        /// <summary>FollowsGround over a hill: it keeps 60 above the ground at every step, and climbs the hill.</summary>
        public static void FollowsGround(FakeWorld world)
        {
            Fresh();
            InstallHill();
            var source = UnitAt(world, 0.0f, 64.0f, 0.0f);
            var dart = new Dart();
            dart.Speed = 512.0f;
            dart.FollowsGround = true;
            source.Fire(dart, 1000.0f, 64.0f);
            var top = 0.0f;
            for (var step = 1; step <= 60; step++)
            {
                Steps(world, 1);
                Within(60.0f, dart.Z - Terrain.Height(dart.X, dart.Y), 0.01f, "above the ground at step " + step);
                if (dart.Z > top)
                {
                    top = dart.Z;
                }
            }

            Within(960.0f, dart.X, 0.05f, "where it is after 60 steps, 16 a step");
            Within(260.0f, top, 0.01f, "it went over the hilltop at 200");
        }

        /// <summary>With Duration 1 it expires after 32 steps: OnExpire once, with the missile as the current owner, then it ends.</summary>
        public static void Duration(FakeWorld world)
        {
            Fresh();
            var dart = new Dart();
            dart.Speed = 100.0f;
            dart.Duration = 1.0f;
            var endedInExpire = true;
            dart.Expiring = () => { endedInExpire = dart.HasEnded; };
            Missile.Create(dart, 0.0f, 0.0f, 100.0f);
            Steps(world, 31);
            Expect.Equal(0, dart.Expired, "no expiry before a second");
            Expect.True(!dart.HasEnded, "flying before a second");
            Steps(world, 1);
            Expect.Equal(1, dart.Expired, "OnExpire after a second");
            Expect.True(!endedInExpire, "it has not ended inside OnExpire");
            Expect.Same(dart, dart.ExpiredWith, "OnExpire runs with the missile");
            Expect.True(dart.HasEnded, "then it ends");
            Steps(world, 4);
            Expect.Equal(1, dart.Expired, "OnExpire once");
        }

        /// <summary>With MaxDistance 450 at 900 a second it expires after 16 steps: OnExpire once, then it ends.</summary>
        public static void MaxDistance(FakeWorld world)
        {
            Fresh();
            var dart = new Dart();
            dart.Speed = 900.0f;
            dart.MaxDistance = 450.0f;
            Missile.Create(dart, 0.0f, 0.0f, 100.0f);
            Steps(world, 15);
            Expect.Equal(0, dart.Expired, "no expiry before 450");
            Expect.True(!dart.HasEnded, "flying before 450");
            Steps(world, 1);
            Expect.Equal(1, dart.Expired, "OnExpire at 450");
            Expect.True(dart.HasEnded, "then it ends");
            Within(450.0f, dart.X, 0.01f, "where it ended");
            Steps(world, 4);
            Expect.Equal(1, dart.Expired, "OnExpire once");
        }

        /// <summary>A missile ends past the world's edge, above 3,000 and below -2,000, with no OnExpire; one inside flies on. The bounds are read once.</summary>
        public static void Bounds(FakeWorld world)
        {
            var fake = Fresh();
            fake.MaxX = 1000.0f;
            var east = new Dart();
            east.Speed = 900.0f;
            Missile.Create(east, 900.0f, 0.0f, 100.0f);
            var up = new Dart();
            up.VZ = 640.0f;
            Missile.Create(up, 0.0f, 0.0f, 2990.0f);
            var down = new Dart();
            down.VZ = -640.0f;
            Missile.Create(down, 0.0f, 0.0f, -1990.0f);
            var inside = new Dart();
            inside.VY = 100.0f;
            Missile.Create(inside, 0.0f, 0.0f, 100.0f);
            Steps(world, 1);
            Expect.True(up.HasEnded, "above 3,000 it ends");
            Expect.True(down.HasEnded, "below -2,000 it ends");
            Expect.True(!east.HasEnded, "inside the edge after a step");
            Steps(world, 2);
            Expect.True(!east.HasEnded, "inside the edge after 3 steps, at 984");
            Steps(world, 1);
            Expect.True(east.HasEnded, "past the edge at 1,012 it ends");
            Expect.Equal(0, east.Expired + up.Expired + down.Expired, "no OnExpire");
            Expect.True(!inside.HasEnded, "the one inside flies on");
            Expect.Equal(1, fake.BoundsReads, "the bounds are read once");
        }

        /// <summary>Fired inside a buff's reaction with no owner, a missile belongs to the game and outlives the buff; given an owner, it ends with it. Create with no owner inside an owner's Run belongs to the game too.</summary>
        public static void Ownership(FakeWorld world)
        {
            Fresh();
            var source = UnitAt(world, 0.0f, 0.0f, 0.0f);
            var volley = source.Apply(new Volley());
            Expect.Same(volley, volley.Seen, "the buff was the current owner when it fired");
            var dart = volley.Fired!;
            Expect.Same(Game.Instance, dart.Owner, "fired with no owner, it belongs to the game");
            volley.End();
            Expect.True(!dart.HasEnded, "the buff ending does not end it");
            Steps(world, 2);
            Expect.True(dart.X > 0.0f, "it flies on");
            var log = new List<string>();
            var owner = new TestOwner("owner", log);
            var other = UnitAt(world, 0.0f, 200.0f, 0.0f);
            var given = new Volley();
            given.Given = owner;
            other.Apply(given);
            var owned = given.Fired!;
            Expect.Same(owner, owned.Owner, "fired with an owner, it belongs to it");
            var placed = new Dart();
            var placedOwned = new Dart();
            owner.Run(() =>
            {
                Missile.Create(placed, 0.0f, 0.0f, 100.0f);
                Missile.Create(placedOwned, 0.0f, 0.0f, 100.0f, Player.Two, owner);
            });
            Expect.Same(Game.Instance, placed.Owner, "Create with no owner, inside an owner's Run, belongs to the game");
            Expect.Same(Player.NeutralPassive, placed.Player, "Neutral Passive unless given a player");
            Expect.Same(owner, placedOwned.Owner, "Create with an owner belongs to it");
            Expect.Same(Player.Two, placedOwned.Player, "the player given");
            owner.End();
            Expect.True(owned.HasEnded, "it ends with its owner");
            Expect.True(placedOwned.HasEnded, "the placed one too");
            Expect.True(!placed.HasEnded && !dart.HasEnded, "the game's are left");
        }

        /// <summary>The effect's yaw and pitch follow the velocity (yaw 0 east, positive pitch nose down, in radians); a missile with no velocity keeps its last facing, and one with FacesFlight off is not turned.</summary>
        public static void Facing(FakeWorld world)
        {
            var fake = Fresh();
            var dart = new Dart();
            dart.Model = "dart.mdl";
            dart.VX = 100.0f;
            dart.VY = 100.0f;
            Missile.Create(dart, 0.0f, 0.0f, 1000.0f);
            Steps(world, 1);
            var last = fake.Orientations.Count - 3;
            Within(Pi / 4.0f, fake.Orientations[last], 0.001f, "yaw north-east");
            Within(0.0f, fake.Orientations[last + 1], 0.001f, "pitch level");
            Within(0.0f, fake.Orientations[last + 2], 0.001f, "no roll");
            dart.Velocity = new Vector3(0.0f, 0.0f, -100.0f);
            Steps(world, 1);
            last = fake.Orientations.Count - 3;
            Within(Pi / 2.0f, fake.Orientations[last + 1], 0.001f, "straight down noses down");
            dart.Velocity = new Vector3(-300.0f, 0.0f, 400.0f);
            Steps(world, 1);
            last = fake.Orientations.Count - 3;
            Within(Pi, Math.Abs(fake.Orientations[last]), 0.001f, "yaw west");
            Within(Climbing, fake.Orientations[last + 1], 0.001f, "climbing noses up");
            dart.Velocity = new Vector3(0.0f, 0.0f, 0.0f);
            var count = fake.Orientations.Count;
            Steps(world, 2);
            Expect.Equal(count, fake.Orientations.Count, "no velocity: not turned");
            Within(Climbing, fake.Orientations[count - 2], 0.001f, "it keeps its last pitch");
            dart.FacesFlight = false;
            dart.VX = 100.0f;
            Steps(world, 2);
            Expect.Equal(count, fake.Orientations.Count, "FacesFlight off: not turned");
        }

        /// <summary>Fired at its own place, a missile has no NaN and flies along the source's facing, level; so at a unit standing there, with an arc, and at a height there.</summary>
        public static void FireAtSelf(FakeWorld world)
        {
            Fresh();
            var source = UnitAt(world, 100.0f, 100.0f, 90.0f);
            var dart = new Dart();
            dart.Speed = 500.0f;
            source.Fire(dart, 100.0f, 100.0f);
            Number(dart.VX, "VX");
            Number(dart.VY, "VY");
            Number(dart.VZ, "VZ");
            Within(0.0f, dart.VX, 0.01f, "no x");
            Within(500.0f, dart.VY, 0.01f, "along the facing, north");
            Within(0.0f, dart.VZ, 0.01f, "level");
            Steps(world, 4);
            Within(162.5f, dart.Y, 0.01f, "it flies north");
            Number(dart.X, "X");
            Number(dart.Z, "Z");
            var target = UnitAt(world, 100.0f, 100.0f, 0.0f);
            var atUnit = new Dart();
            atUnit.Speed = 500.0f;
            source.Fire(atUnit, target);
            Within(500.0f, atUnit.VY, 0.01f, "at a unit on the spot: along the facing");
            Number(atUnit.VZ, "at a unit on the spot: VZ");
            var arced = new Dart();
            arced.Speed = 500.0f;
            arced.Arc = 0.35f;
            source.Fire(arced, 100.0f, 100.0f);
            Number(arced.VZ, "with an arc: VZ");
            Number(arced.Gravity, "with an arc: Gravity");
            Within(500.0f, arced.VY, 0.01f, "with an arc: along the facing");
            var high = new Dart();
            high.Speed = 500.0f;
            source.Fire(high, 100.0f, 100.0f, 300.0f);
            Number(high.VZ, "at a height on the spot: VZ");
            Within(500.0f, high.VY, 0.01f, "at a height on the spot: along the facing");
            Steps(world, 2);
            Number(arced.Z, "with an arc: Z after 2 steps");
        }

        /// <summary>Model, Scale, ModelHeight, Tint and AddEffect make the fake's effects, set before Create or after; they move together, a new Model replaces the effect, and all are destroyed at the end.</summary>
        public static void Effects(FakeWorld world)
        {
            var fake = Fresh();
            var dart = new Dart();
            dart.Model = "dart.mdl";
            dart.Scale = 2.0f;
            dart.ModelHeight = 30.0f;
            dart.Tint(255, 0, 0, 128);
            dart.AddEffect("glow.mdl", 10.0f);
            dart.VX = 320.0f;
            Expect.True(dart.Effect == null, "no effect before Create");
            Expect.Equal(0, fake.Effects.Count, "none made before Create");
            Missile.Create(dart, 0.0f, 0.0f, 100.0f);
            Expect.Equal(2, fake.Effects.Count, "the model and the added effect");
            var main = dart.Effect!;
            var extra = fake.Effects[0] == main ? fake.Effects[1] : fake.Effects[0];
            Expect.Equal("dart.mdl", fake.Models[main], "the model");
            Expect.Equal("glow.mdl", fake.Models[extra], "the added model");
            Within(2.0f, fake.Scales[main], 0.001f, "the scale");
            Expect.Equal(255, fake.Colors[main][0], "red");
            Expect.Equal(0, fake.Colors[main][1], "green");
            Expect.Equal(0, fake.Colors[main][2], "blue");
            Expect.Equal(128, fake.Alphas[main], "alpha");
            Within(130.0f, fake.Places[main][2], 0.001f, "the model is drawn 30 above its place");
            Steps(world, 1);
            Within(10.0f, fake.Places[main][0], 0.001f, "the model moved: x");
            Within(130.0f, fake.Places[main][2], 0.001f, "the model moved: z");
            Within(10.0f, fake.Places[extra][0], 0.001f, "the added effect moved with it: x");
            Within(110.0f, fake.Places[extra][2], 0.001f, "the added effect is 10 above its place");
            dart.AddEffect("spark.mdl", 0.0f);
            Expect.Equal(3, fake.Effects.Count, "an effect added after Create is made at once");
            var spark = fake.Effects[2];
            Steps(world, 1);
            Within(20.0f, fake.Places[spark][0], 0.001f, "and moves with it");
            Within(100.0f, fake.Places[spark][2], 0.001f, "at its height");
            dart.Scale = 3.0f;
            Within(3.0f, fake.Scales[main], 0.001f, "a scale set later");
            dart.Tint(0, 255, 0);
            Expect.Equal(255, fake.Colors[main][1], "a tint set later");
            Expect.Equal(255, fake.Alphas[main], "alpha 255 unless given");
            dart.Model = "other.mdl";
            var replaced = dart.Effect!;
            Expect.True(replaced != main, "a new model makes a new effect");
            Expect.True(!fake.Effects.Contains(main), "and destroys the old one");
            Expect.Equal("other.mdl", fake.Models[replaced], "of the new model");
            Within(3.0f, fake.Scales[replaced], 0.001f, "with the scale");
            Expect.Equal(255, fake.Colors[replaced][1], "with the tint");
            Within(20.0f, fake.Places[replaced][0], 0.001f, "where the missile is");
            Within(130.0f, fake.Places[replaced][2], 0.001f, "at the model height");
            Expect.Equal(3, fake.Effects.Count, "three effects");
            dart.End();
            Expect.Equal(0, fake.Effects.Count, "all destroyed at the end");
            Expect.True(dart.Effect == null, "no effect after the end");
            var plain = new Dart();
            Missile.Create(plain, 0.0f, 0.0f, 0.0f);
            Expect.True(plain.Effect == null, "no model, no effect");
            Expect.Equal(0, fake.Effects.Count, "none made");
        }
    }
}
