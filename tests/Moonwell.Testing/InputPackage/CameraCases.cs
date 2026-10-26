using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>Cases for each player's camera and World Editor's cameras: the machine-local reads and their units, the changes that reach one player's machine, and the setups made at Install.</summary>
    public static class CameraCases
    {
        /// <summary>Adds one camera's numbers as the build writes them: target x and y, the count of pairs, the pairs (field code, value), then the type.</summary>
        private static void AddCamera(List<float> data, float x, float y, int[] codes, float[] values, int type)
        {
            data.Add(x);
            data.Add(y);
            data.Add(codes.Length);
            for (var i = 0; i < codes.Length; i++)
            {
                data.Add(codes[i]);
                data.Add(values[i]);
            }

            data.Add(type);
        }

        /// <summary>Makes slots 0 and 1 user players and installs the package on a fake world, with the cameras given.</summary>
        private static FakeInputWorld Fresh(FakeWorld world, string[] names, float[] cameras)
        {
            world.SetSlot(0, SlotState.Playing, Controller.User);
            world.SetSlot(1, SlotState.Playing, Controller.User);
            var fake = new FakeInputWorld();
            InputSystem.Install(fake, false, names, cameras);
            return fake;
        }

        /// <summary>Installs with no cameras.</summary>
        private static FakeInputWorld Fresh(FakeWorld world)
        {
            return Fresh(world, new string[0], new float[0]);
        }

        /// <summary>The data of two cameras: "Intro" with seven pairs and no type, "Outro" with fourteen pairs and a type.</summary>
        private static float[] TwoCameras()
        {
            var data = new List<float>();
            AddCamera(data, 100f, 200f, new[] { 6, 5, 2, 0, 4, 3, 1 }, new[] { 10f, 90f, 304f, 1650f, 0f, 70f, 5000f }, -1);
            AddCamera(
                data,
                -300f,
                400f,
                new[] { 6, 5, 2, 0, 4, 3, 1, 7, 8, 9, 10, 11, 12, 13 },
                new[] { 1f, 2f, 3f, 4f, 5f, 6f, 7f, 8f, 9f, 10f, 11f, 12f, 13f, 14f },
                2);
            return data.ToArray();
        }

        /// <summary>The reads give the game's numbers: the angles in degrees (the game gives radians), the rest as they are.</summary>
        public static void Reads(FakeWorld world)
        {
            var fake = Fresh(world);
            fake.Fields[0] = 1650f;
            fake.Fields[1] = 5300f;
            fake.Fields[2] = 5.236f;
            fake.Fields[3] = 1.0472f;
            fake.Fields[4] = 0.5236f;
            fake.Fields[5] = 3.1416f;
            fake.Fields[6] = 10f;
            fake.TargetX = 11f;
            fake.TargetY = 12f;
            fake.TargetZ = 13f;
            fake.EyeX = 21f;
            fake.EyeY = 22f;
            fake.EyeZ = 23f;
            var camera = Player.One.Camera();
            Moonwell.Local.For(Player.One, () =>
            {
                Expect.Near(1650f, camera.Distance, "Distance");
                Expect.Near(5300f, camera.FarZ, "FarZ");
                Expect.Near(300f, camera.AngleOfAttack, "AngleOfAttack in degrees");
                Expect.Near(60f, camera.FieldOfView, "FieldOfView in degrees");
                Expect.Near(30f, camera.Roll, "Roll in degrees");
                Expect.Near(180f, camera.Rotation, "Rotation in degrees");
                Expect.Near(10f, camera.ZOffset, "ZOffset");
                Expect.Near(11f, camera.TargetX, "TargetX");
                Expect.Near(12f, camera.TargetY, "TargetY");
                Expect.Near(13f, camera.TargetZ, "TargetZ");
                Expect.Near(11f, camera.Target.X, "Target.X");
                Expect.Near(12f, camera.Target.Y, "Target.Y");
                Expect.Near(13f, camera.Target.Z, "Target.Z");
                Expect.Near(21f, camera.EyeX, "EyeX");
                Expect.Near(22f, camera.EyeY, "EyeY");
                Expect.Near(23f, camera.EyeZ, "EyeZ");
                Expect.Near(21f, camera.Eye.X, "Eye.X");
                Expect.Near(22f, camera.Eye.Y, "Eye.Y");
                Expect.Near(23f, camera.Eye.Z, "Eye.Z");
            });
            Expect.True(camera.Player == Player.One, "the camera's player");
            Expect.True(Player.Two.Camera().Player == Player.Two, "another player's camera is its own");
        }

        /// <summary>Each change reaches the world with the camera's player, and the angles pass in degrees.</summary>
        public static void Changes(FakeWorld world)
        {
            var fake = Fresh(world);
            var one = Player.One.Camera();
            one.SetAngleOfAttack(300f, 1f);
            one.PanTo(new Vector2(5f, 6f), 2f);
            one.PanTo(7f, 8f, 3f);
            one.SetDistance(1800f);
            one.SetRotation(90f, 0.5f);
            one.SetRoll(5f);
            one.SetFieldOfView(70f, 4f);
            one.SetFarZ(6000f);
            one.SetZOffset(50f, 1.5f);
            var unit = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            one.Follow(Unit.Of(unit)!);
            one.StopFollowing();
            Player.Two.Camera().Reset(2.5f);
            one.Reset();
            Player.Two.Camera().SetBounds(new Bounds(new Vector2(-100f, -200f), new Vector2(300f, 400f)));
            var calls = fake.CameraCalls;
            Expect.Equal(14, calls.Count, "the number of changes");
            ExpectField(calls[0], 0, 2, 300f, 1f, "angle of attack");
            Expect.Equal("pan", calls[1].Kind, "pan 1 kind");
            Expect.Equal(0, calls[1].Slot, "pan 1 slot");
            Expect.Near(5f, calls[1].A, "pan 1 x");
            Expect.Near(6f, calls[1].B, "pan 1 y");
            Expect.Near(2f, calls[1].Seconds, "pan 1 seconds");
            Expect.Equal("pan", calls[2].Kind, "pan 2 kind");
            Expect.Near(7f, calls[2].A, "pan 2 x");
            Expect.Near(8f, calls[2].B, "pan 2 y");
            Expect.Near(3f, calls[2].Seconds, "pan 2 seconds");
            ExpectField(calls[3], 0, 0, 1800f, 0f, "distance");
            ExpectField(calls[4], 0, 5, 90f, 0.5f, "rotation");
            ExpectField(calls[5], 0, 4, 5f, 0f, "roll");
            ExpectField(calls[6], 0, 3, 70f, 4f, "field of view");
            ExpectField(calls[7], 0, 1, 6000f, 0f, "far Z");
            ExpectField(calls[8], 0, 6, 50f, 1.5f, "Z offset");
            Expect.Equal("follow", calls[9].Kind, "follow kind");
            Expect.Equal(0, calls[9].Slot, "follow slot");
            Expect.Same(unit, calls[9].Unit, "follow unit");
            Expect.Equal("stop", calls[10].Kind, "stop kind");
            Expect.Equal(0, calls[10].Slot, "stop slot");
            Expect.Equal("reset", calls[11].Kind, "reset 1 kind");
            Expect.Equal(1, calls[11].Slot, "reset 1 slot");
            Expect.Near(2.5f, calls[11].Seconds, "reset 1 seconds");
            Expect.Equal("reset", calls[12].Kind, "reset 2 kind");
            Expect.Equal(0, calls[12].Slot, "reset 2 slot");
            Expect.Near(0f, calls[12].Seconds, "reset 2 seconds");
            Expect.Equal("bounds", calls[13].Kind, "bounds kind");
            Expect.Equal(1, calls[13].Slot, "bounds slot");
            Expect.Near(-100f, calls[13].A, "bounds min x");
            Expect.Near(-200f, calls[13].B, "bounds min y");
            Expect.Near(300f, calls[13].C, "bounds max x");
            Expect.Near(400f, calls[13].D, "bounds max y");
        }

        /// <summary>Fails unless a change is a field change of this player, with this code, value and seconds.</summary>
        private static void ExpectField(FakeInputWorld.CameraCall call, int slot, int field, float value, float seconds, string what)
        {
            Expect.Equal("field", call.Kind, what + " kind");
            Expect.Equal(slot, call.Slot, what + " slot");
            Expect.Equal(field, call.Field, what + " field code");
            Expect.Near(value, call.A, what + " value");
            Expect.Near(seconds, call.Seconds, what + " seconds");
        }

        /// <summary>The setups are made at Install, one for each name, from the numbers of that camera; Named gives the same one each time, and Apply reaches the world.</summary>
        public static void Named(FakeWorld world)
        {
            var fake = Fresh(world, new[] { "Intro", "Outro" }, TwoCameras());
            Expect.Equal(2, fake.Made.Count, "one setup for each camera");
            var first = fake.Made[0];
            Expect.Near(100f, first.X, "Intro x");
            Expect.Near(200f, first.Y, "Intro y");
            Expect.Equal(7, first.Codes.Count, "Intro's pairs");
            Expect.Equal(6, first.Codes[0], "Intro's first code");
            Expect.Near(10f, first.Values[0], "Intro's first value");
            Expect.Equal(0, first.Codes[3], "Intro's fourth code");
            Expect.Near(1650f, first.Values[3], "Intro's fourth value");
            Expect.Equal(1, first.Codes[6], "Intro's last code");
            Expect.Near(5000f, first.Values[6], "Intro's last value");
            Expect.Equal(-1, first.Type, "Intro has no type");
            var second = fake.Made[1];
            Expect.Near(-300f, second.X, "Outro x");
            Expect.Near(400f, second.Y, "Outro y");
            Expect.Equal(14, second.Codes.Count, "Outro's pairs");
            Expect.Equal(13, second.Codes[13], "Outro's last code");
            Expect.Near(14f, second.Values[13], "Outro's last value");
            Expect.Equal(2, second.Type, "Outro's type");

            var intro = CameraSetup.Named("Intro");
            Expect.Same(intro, CameraSetup.Named("Intro"), "the same setup each time");
            Expect.True(intro != CameraSetup.Named("Outro"), "another name is another setup");
            Player.Two.Camera().Apply(intro, 1.5f);
            Player.One.Camera().Apply(CameraSetup.Named("Outro"));
            Expect.Equal(2, fake.CameraCalls.Count, "two applies");
            Expect.Equal("apply", fake.CameraCalls[0].Kind, "apply kind");
            Expect.Equal(1, fake.CameraCalls[0].Slot, "apply slot");
            Expect.Same(first.Setup, fake.CameraCalls[0].Setup, "apply used Intro's setup");
            Expect.Near(1.5f, fake.CameraCalls[0].Seconds, "apply seconds");
            Expect.Equal(0, fake.CameraCalls[1].Slot, "second apply slot");
            Expect.Same(second.Setup, fake.CameraCalls[1].Setup, "second apply used Outro's setup");
            Expect.Near(0f, fake.CameraCalls[1].Seconds, "second apply seconds");
        }

        /// <summary>A name no camera has throws, naming it.</summary>
        public static void UnknownCamera(FakeWorld world)
        {
            Fresh(world, new[] { "Intro", "Outro" }, TwoCameras());
            Expect.Throws<ArgumentException>(() => CameraSetup.Named("Nowhere"), "an unknown camera");
            string? message = null;
            try
            {
                CameraSetup.Named("Nowhere");
            }
            catch (ArgumentException e)
            {
                message = e.Message;
            }

            Expect.Equal("No World Editor camera is named 'Nowhere'.", message, "the message");
            Fresh(world);
            Expect.Throws<ArgumentException>(() => CameraSetup.Named("Intro"), "a map with no cameras");
        }

        /// <summary>Install throws ArgumentException unless the numbers hold exactly the cameras the names say, and makes nothing then.</summary>
        public static void WrongData(FakeWorld world)
        {
            world.SetSlot(0, SlotState.Playing, Controller.User);
            var good = TwoCameras();
            var fake = new FakeInputWorld();
            Expect.Throws<ArgumentException>(() => InputSystem.Install(fake, false, new[] { "Intro" }, good), "more numbers than the names hold");
            Expect.Throws<ArgumentException>(() => InputSystem.Install(fake, false, new[] { "Intro", "Outro", "Third" }, good), "a name too many");
            Expect.Throws<ArgumentException>(() => InputSystem.Install(fake, false, new[] { "Intro", "Outro" }, new float[0]), "names and no numbers");
            Expect.Throws<ArgumentException>(() => InputSystem.Install(fake, false, new string[0], good), "numbers and no names");
            var cut = new float[good.Length - 1];
            for (var i = 0; i < cut.Length; i++)
            {
                cut[i] = good[i];
            }

            Expect.Throws<ArgumentException>(() => InputSystem.Install(fake, false, new[] { "Intro", "Outro" }, cut), "the last camera without its type");
            var short2 = new float[] { 1f, 2f };
            Expect.Throws<ArgumentException>(() => InputSystem.Install(fake, false, new[] { "Intro" }, short2), "a camera cut before its count");
            Expect.Equal(0, fake.Made.Count, "no setup was made from bad data");
        }

        /// <summary>A camera read before Install throws, and so does a setup; a map's cameras are forgotten when Install runs again.</summary>
        public static void NotInstalled(FakeWorld world)
        {
            Fresh(world, new[] { "Intro", "Outro" }, TwoCameras());
            InputSystem.Uninstall();
            Expect.Throws<InvalidOperationException>(() => Player.One.Camera(), "a camera before Install");
            Expect.Throws<InvalidOperationException>(() => CameraSetup.Named("Intro"), "a setup before Install");
            Fresh(world);
            Expect.Throws<ArgumentException>(() => CameraSetup.Named("Intro"), "the earlier install's cameras are gone");
        }
    }
}
