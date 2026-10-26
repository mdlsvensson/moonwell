using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>Cases for the missile components the package ships, Homing, Shadow and LoopingSound, with the default step of 1/32 second on flat ground at 0. A footman of the fake reads a collision size of 16, so it is 64 tall and its middle is 32 up.</summary>
    public static class ComponentCases
    {
        /// <summary>A footman's middle above the ground in the fake: half of 4 times its collision size of 16.</summary>
        private const float Middle = 32.0f;

        /// <summary>The game's unit shadow.</summary>
        private const string UnitShadow = @"ReplaceableTextures\Shadows\Shadow.blp";

        /// <summary>A sound path for the sound cases.</summary>
        private const string Hum = @"Abilities\Spells\Undead\Possession\PossessionMissileLaunch1.wav";

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
            Expect.True(!float.IsNaN(value) && !float.IsInfinity(value), what + " is a number");
        }

        /// <summary>Fails when any of a missile's place or velocity is not a number.</summary>
        private static void Numbers(Missile missile, string what)
        {
            Number(missile.X, what + ": X");
            Number(missile.Y, what + ": Y");
            Number(missile.Z, what + ": Z");
            Number(missile.VX, what + ": VX");
            Number(missile.VY, what + ": VY");
            Number(missile.VZ, what + ": VZ");
        }

        /// <summary>The angle between two directions, in degrees, from the length of their cross product and their dot product, which is exact near 0 where an arc cosine is not.</summary>
        private static float Between(float ax, float ay, float az, float bx, float by, float bz)
        {
            var cx = ay * bz - az * by;
            var cy = az * bx - ax * bz;
            var cz = ax * by - ay * bx;
            var cross = MathF.Sqrt(cx * cx + cy * cy + cz * cz);
            var dot = ax * bx + ay * by + az * bz;
            return MathF.Atan2(cross, dot) * 180.0f / MathF.PI;
        }

        /// <summary>A target behind a missile at 600 with TurnRate 180: its heading changes by at most 18 degrees in any 0.1 second, at its speed, and it has come round to the target after a second.</summary>
        public static void TurnRate(FakeWorld world)
        {
            Fresh();
            var target = UnitAt(world, -1000.0f, 0.0f, 0.0f);
            var dart = new Dart();
            dart.VX = 600.0f;
            Missile.Create(dart, 0.0f, 0.0f, Middle);
            var homing = dart.Add(new Homing { Target = target, TurnRate = 180.0f });
            Expect.Same(target, homing.Target, "the target");
            Within(180.0f, homing.TurnRate, 0.0f, "the turn rate");
            Expect.True(!homing.EndsWithTarget, "EndsWithTarget is false unless set");
            var changes = new List<float>();
            var vx = dart.VX;
            var vy = dart.VY;
            var vz = dart.VZ;
            var turns = 0;
            for (var step = 1; step <= 32; step++)
            {
                Steps(world, 1);
                var change = Between(vx, vy, vz, dart.VX, dart.VY, dart.VZ);
                changes.Add(change);
                if (change > 0.001f)
                {
                    turns++;
                }

                Within(600.0f, dart.Speed, 0.01f, "it keeps its speed at step " + step);
                vx = dart.VX;
                vy = dart.VY;
                vz = dart.VZ;
            }

            for (var i = 0; i + 2 < changes.Count; i++)
            {
                // Three steps are 0.09375 seconds.
                var turned = changes[i] + changes[i + 1] + changes[i + 2];
                Expect.True(turned <= 18.01f, "at most 18 degrees in 0.1 second, from step " + (i + 1) + ": was " + turned);
            }

            Expect.True(turns >= 9 && turns <= 11, "it turns about 10 times a second; was " + turns);
            var toward = Between(dart.VX, dart.VY, dart.VZ, target.X - dart.X, target.Y - dart.Y, Middle - dart.Z);
            Expect.True(toward < 20.0f, "after a second it flies at the target; " + toward + " degrees off");
        }

        /// <summary>A target inside the circle a missile at 600 cannot turn into at TurnRate 90 (radius 382): it flies on straight, then turns and reaches it within 5 seconds, with no orbit.</summary>
        public static void Circle(FakeWorld world)
        {
            Fresh();
            var target = UnitAt(world, 0.0f, 300.0f, 0.0f);
            var dart = new Dart();
            dart.VX = 600.0f;
            Missile.Create(dart, 0.0f, 0.0f, Middle);
            dart.Add(new Homing { Target = target, TurnRate = 90.0f });
            Steps(world, 4);
            Within(600.0f, dart.VX, 0.001f, "it flies on straight: x");
            Within(0.0f, dart.VY, 0.001f, "it flies on straight: y");
            var nearest = 100000.0f;
            for (var step = 1; step <= 156; step++)
            {
                Steps(world, 1);
                var dx = dart.X - target.X;
                var dy = dart.Y - target.Y;
                var dz = dart.Z - Middle;
                var distance = MathF.Sqrt(dx * dx + dy * dy + dz * dz);
                if (distance < nearest)
                {
                    nearest = distance;
                }
            }

            Expect.True(nearest < 20.0f, "it reaches the target within 5 seconds; nearest " + nearest);
        }

        /// <summary>A target that dies: the missile flies on straight and Target reads null. With EndsWithTarget, a target removed from the game ends it at its next turn, and nothing reads the removed unit. A gizmo target that ends: straight on.</summary>
        public static void Lost(FakeWorld world)
        {
            Fresh();
            var dead = UnitAt(world, 1000.0f, 0.0f, 0.0f);
            var dart = new Dart();
            dart.VY = 600.0f;
            Missile.Create(dart, 0.0f, 0.0f, Middle);
            var homing = dart.Add(new Homing { Target = dead, TurnRate = 90.0f });
            Steps(world, 8);
            Expect.True(dart.VX > 0.0f, "it has begun to turn to the target");
            dead.Kill();
            var vx = dart.VX;
            var vy = dart.VY;
            var vz = dart.VZ;
            Steps(world, 32);
            Within(vx, dart.VX, 0.001f, "straight on after the death: x");
            Within(vy, dart.VY, 0.001f, "straight on after the death: y");
            Within(vz, dart.VZ, 0.001f, "straight on after the death: z");
            Expect.True(homing.Target == null, "Target reads null once the target is lost");
            Expect.True(!dart.HasEnded, "it flies on");
            Expect.True(!homing.HasEnded, "the Homing stays on it");

            var gone = UnitAt(world, 1000.0f, 500.0f, 0.0f);
            var ender = new Dart();
            ender.VY = 600.0f;
            Missile.Create(ender, 0.0f, 500.0f, Middle);
            ender.Add(new Homing { Target = gone, TurnRate = 90.0f, EndsWithTarget = true });
            Steps(world, 2);
            Expect.True(!ender.HasEnded, "it flies while the target is there");
            gone.Remove();
            var calls = world.GoneCalls;
            Steps(world, 4);
            Expect.True(ender.HasEnded, "with EndsWithTarget it ends at its next turn after the target is removed");
            Expect.Equal(calls, world.GoneCalls, "nothing reads the removed unit");

            var ball = new Dart();
            Missile.Create(ball, 1000.0f, -500.0f, Middle);
            var follower = new Dart();
            follower.VY = -600.0f;
            Missile.Create(follower, 0.0f, -500.0f, Middle);
            var follows = follower.Add(new Homing { Target = ball, TurnRate = 90.0f });
            Steps(world, 8);
            Expect.True(follower.VX > 0.0f, "it turns to the gizmo");
            ball.End();
            vx = follower.VX;
            vy = follower.VY;
            Steps(world, 16);
            Within(vx, follower.VX, 0.001f, "straight on after the gizmo ends: x");
            Within(vy, follower.VY, 0.001f, "straight on after the gizmo ends: y");
            Expect.True(follows.Target == null && !follower.HasEnded, "the gizmo is let go and it flies on");
        }

        /// <summary>Fire at a unit gives the missile's Homing that unit as its target; Fire at a point gives it none.</summary>
        public static void FireSetsTarget(FakeWorld world)
        {
            Fresh();
            var source = UnitAt(world, 0.0f, 0.0f, 0.0f);
            var target = UnitAt(world, 500.0f, 200.0f, 0.0f);
            var seeker = new HomingDart(180.0f);
            seeker.Speed = 600.0f;
            source.Fire(seeker, target);
            var homing = seeker.Get<Homing>();
            Expect.True(homing != null, "the Homing listed in its constructor is on it");
            Expect.Same(target, homing!.Target, "Fire at a unit sets its target");
            Within(180.0f, homing.TurnRate, 0.0f, "its turn rate");
            var aimless = new HomingDart(180.0f);
            aimless.Speed = 600.0f;
            source.Fire(aimless, 500.0f, 0.0f);
            Expect.True(aimless.Get<Homing>()!.Target == null, "Fire at a point gives it no target");
            Steps(world, 4);
            Within(600.0f, aimless.Speed, 0.01f, "with no target it flies on");
            Within(0.0f, aimless.VY, 0.001f, "straight");
        }

        /// <summary>An arc fired with Homing at a unit walking 120 a second across it re-aims the arc's rest at the unit: it comes down to the unit's middle within 16 of where the unit is then, well away from where it was at launch.</summary>
        public static void Arc(FakeWorld world)
        {
            Fresh();
            var source = UnitAt(world, 0.0f, 0.0f, 0.0f);
            var walker = UnitAt(world, 800.0f, 0.0f, 90.0f);
            var shell = new HomingDart(0.0f);
            shell.Speed = 700.0f;
            shell.Arc = 0.35f;
            source.Fire(shell, walker);
            var walk = 120.0f * Interactions.Step;
            var walkerY = 0.0f;
            var previousX = shell.X;
            var previousY = shell.Y;
            var previousZ = shell.Z;
            var landed = false;
            for (var step = 1; step <= 96; step++)
            {
                walkerY += walk;
                world.MoveUnit(walker.Handle, 800.0f, walkerY);
                Steps(world, 1);
                Numbers(shell, "the shell at step " + step);
                if (step > 4 && shell.Z <= Middle && previousZ > Middle)
                {
                    var share = (previousZ - Middle) / (previousZ - shell.Z);
                    var landX = previousX + (shell.X - previousX) * share;
                    var landY = previousY + (shell.Y - previousY) * share;
                    var dx = landX - 800.0f;
                    var dy = landY - walkerY;
                    var off = MathF.Sqrt(dx * dx + dy * dy);
                    Expect.True(off <= 16.0f, "it comes down within 16 of the unit; was " + off + " at step " + step);
                    Expect.True(walkerY > 100.0f, "the unit has walked away from where it was at launch");
                    landed = true;
                    break;
                }

                previousX = shell.X;
                previousY = shell.Y;
                previousZ = shell.Z;
            }

            Expect.True(landed, "it came down");
        }

        /// <summary>A target at the missile's own place: no turn makes a number that is not one, and the speed is kept; the same for an arc, and for a still missile.</summary>
        public static void AtTarget(FakeWorld world)
        {
            Fresh();
            var target = UnitAt(world, 500.0f, 0.0f, 0.0f);
            var dart = new Dart();
            dart.VX = 600.0f;
            Missile.Create(dart, 500.0f, 0.0f, Middle);
            dart.Add(new Homing { Target = target, TurnRate = 180.0f });
            Steps(world, 1);
            Numbers(dart, "at the target's place");
            Within(600.0f, dart.VX, 0.001f, "it flies on at its speed");
            Steps(world, 8);
            Numbers(dart, "after");
            Within(600.0f, dart.Speed, 0.01f, "its speed");

            var lob = new Dart();
            lob.VX = 300.0f;
            lob.Arc = 0.35f;
            lob.Gravity = 800.0f;
            Missile.Create(lob, 500.0f, 0.0f, Middle);
            lob.Add(new Homing { Target = target });
            Steps(world, 9);
            Numbers(lob, "an arc at the target's place");

            var still = new Dart();
            Missile.Create(still, 500.0f, 0.0f, 100.0f);
            still.Add(new Homing { Target = target });
            Steps(world, 9);
            Numbers(still, "a still missile");
            Within(0.0f, still.Speed, 0.0f, "a still missile is not turned into motion");
        }

        /// <summary>A shadow is the game's unit shadow, twice the radius across, centred under the missile as it moves, at full alpha on the ground and fainter as it climbs, and destroyed when the missile ends. Its settings are used; one removed mid-flight takes its image away.</summary>
        public static void ShadowFollows(FakeWorld world)
        {
            var fake = Fresh();
            var dart = new Dart();
            dart.VX = 320.0f;
            dart.VZ = 320.0f;
            Missile.Create(dart, 0.0f, 0.0f, 16.0f);
            var shadow = dart.Add(new Shadow());
            Expect.Equal(1, fake.Images.Count, "an image is made");
            var image = fake.Images[0];
            Expect.Equal(UnitShadow, fake.ImageFiles[image], "the game's unit shadow");
            Within(32.0f, fake.ImageSizes[image], 0.001f, "twice the radius across");
            Within(32.0f, shadow.Size, 0.001f, "Size reads twice the radius");
            Expect.Equal(255, fake.ImageAlphas[image], "full alpha on the ground");
            Within(0.0f, fake.ImagePlaces[image][0], 0.001f, "made under it: x");
            Steps(world, 16);
            Within(dart.X, fake.ImagePlaces[image][0], 0.001f, "under it: x");
            Within(dart.Y, fake.ImagePlaces[image][1], 0.001f, "under it: y");
            var low = fake.ImageAlphas[image];
            Expect.True(low < 255 && low > 0, "fainter above the ground; was " + low);
            Steps(world, 16);
            Within(dart.X, fake.ImagePlaces[image][0], 0.001f, "still under it: x");
            var high = fake.ImageAlphas[image];
            Expect.True(high < low, "fainter higher up; " + high + " against " + low);
            dart.End();
            Expect.Equal(0, fake.Images.Count, "the image is destroyed at the end");
            Expect.True(fake.RemovedImages.Contains(image), "the image is removed");

            var second = new Dart();
            second.VX = 100.0f;
            Missile.Create(second, 0.0f, 500.0f, 16.0f);
            var set = second.Add(new Shadow { Path = @"Textures\Other.blp", Size = 100.0f, Alpha = 128 });
            var other = fake.Images[0];
            Expect.Equal(@"Textures\Other.blp", fake.ImageFiles[other], "its path");
            Within(100.0f, fake.ImageSizes[other], 0.001f, "its size");
            Expect.Equal(128, fake.ImageAlphas[other], "its alpha on the ground");
            Expect.Equal(128, set.Alpha, "Alpha reads back");
            second.Remove<Shadow>();
            Expect.Equal(0, fake.Images.Count, "removed from the missile, its image is destroyed");
            Steps(world, 4);
            Expect.True(!second.HasEnded && second.X > 0.0f, "the missile flies on");
        }

        /// <summary>A looping sound starts at the missile at full volume, is moved with it each step, and fades out over its FadeOut after the missile ends, then stops; the fade outlives the missile and its owner. With FadeOut 0 it stops at once.</summary>
        public static void SoundFollows(FakeWorld world)
        {
            var fake = Fresh();
            var log = new List<string>();
            var owner = new TestOwner("owner", log);
            var dart = new Dart();
            dart.VX = 300.0f;
            Missile.Create(dart, 0.0f, 0.0f, 100.0f, null, owner);
            var hum = dart.Add(new LoopingSound(Hum));
            Expect.Equal(Hum, hum.Path, "its path");
            Within(100.0f, hum.Volume, 0.0f, "Volume is 100 unless set");
            Within(0.5f, hum.FadeOut, 0.0f, "FadeOut is half a second unless set");
            Expect.Equal(1, fake.Loops.Count, "a looping sound is made");
            var sound = fake.Loops[0];
            Expect.Equal(Hum, fake.SoundPaths[sound], "of its path");
            Expect.Equal(127, fake.Volumes[sound], "at full volume");
            Within(100.0f, fake.SoundPlaces[sound][2], 0.001f, "at the missile's height");
            var moves = fake.SoundMoves;
            Steps(world, 8);
            Expect.Equal(moves + 8, fake.SoundMoves, "moved each step");
            Within(dart.X, fake.SoundPlaces[sound][0], 0.001f, "with it: x");
            Within(dart.Y, fake.SoundPlaces[sound][1], 0.001f, "with it: y");
            Within(dart.Z, fake.SoundPlaces[sound][2], 0.001f, "with it: z");
            owner.End();
            Expect.True(dart.HasEnded && hum.HasEnded, "the missile and its sound end with the owner");
            Expect.Equal(1, fake.Loops.Count, "the sound plays on as it fades");
            Steps(world, 8);
            var fading = fake.Volumes[sound];
            Expect.True(fading < 127 && fading > 0, "quieter after a quarter second; was " + fading);
            Expect.Equal(1, fake.Loops.Count, "not yet stopped");
            Steps(world, 12);
            Expect.Equal(0, fake.Loops.Count, "stopped after its FadeOut");
            Expect.True(fake.Stopped.Contains(sound), "the sound was stopped");

            var quiet = new Dart();
            Missile.Create(quiet, 0.0f, 0.0f, 100.0f);
            quiet.Add(new LoopingSound(Hum) { Volume = 50.0f, FadeOut = 0.0f });
            var halved = fake.Loops[0];
            Expect.Equal(64, fake.Volumes[halved], "half volume is 64 of 127");
            quiet.End();
            Expect.Equal(0, fake.Loops.Count, "with FadeOut 0 it stops at once");
        }
    }
}
