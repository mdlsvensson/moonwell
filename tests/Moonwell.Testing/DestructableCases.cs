namespace Moonwell.Testing
{
    /// <summary>Cases that check destructables: their classes, the watch on their deaths, and the sweep.</summary>
    public static class DestructableCases
    {
        /// <summary>Checks that the log is exactly the expected lines, in order.</summary>
        private static void ExpectLog(string[] expected, string what)
        {
            Expect.Equal(expected.Length, TestLog.Lines.Count, what + ": line count");
            for (var i = 0; i < expected.Length; i++)
            {
                Expect.Equal(expected[i], TestLog.Lines[i], what + ": line " + i);
            }
        }

        /// <summary>A destructable is watched for its death when its class or a component overrides OnDeath, and its death runs the type, then the components.</summary>
        public static void Watch(FakeWorld world)
        {
            ObjectTypes.Destructable(ObjectTypes.FourCC("B001"), () => new TestGate());
            var gateHandle = world.NewDestructable(ObjectTypes.FourCC("B001"));
            var gate = Destructable.Of(gateHandle)!;
            Expect.True(gate is TestGate, "the destructable is a TestGate");
            Expect.Equal(1, world.Watched.Count, "the deaths asked to hear");
            Expect.Same(gateHandle, world.Watched[0], "the gate watched");

            var plainHandle = world.NewDestructable(ObjectTypes.FourCC("dfoo"));
            var plain = Destructable.Of(plainHandle)!;
            Expect.Equal(1, world.Watched.Count, "a plain destructable is not watched yet");
            plain.Add(new GatePart());
            Expect.Equal(2, world.Watched.Count, "the deaths asked to hear after a component");
            Expect.Same(plainHandle, world.Watched[1], "the plain destructable watched");

            TestLog.Lines.Clear();
            Inbound.DestructableDied(gateHandle);
            ExpectLog(new string[] { "gate died", "part died" }, "the log");
        }

        /// <summary>A destructable that vanishes is ended by the sweep, and the game is not asked to remove it.</summary>
        public static void SweepDetectsRemoval(FakeWorld world)
        {
            var handle = world.NewDestructable(ObjectTypes.FourCC("dfoo"));
            var found = Destructable.Of(handle)!;
            world.VanishDestructable(handle);
            world.Advance(0.3f);
            Expect.True(found.HasEnded, "the vanished destructable has ended");
            Expect.True(found.IsGone, "the vanished destructable is gone");
            Expect.Equal(0, world.RemovedDestructables.Count, "the destructables the game was asked to remove");
        }

        /// <summary>A destructable killed and then removed runs no death reaction from the removal: the game fires the death event again inside RemoveDestructable.</summary>
        public static void RemoveAfterDeath(FakeWorld world)
        {
            world.DeathOnRemove = true;
            ObjectTypes.Destructable(ObjectTypes.FourCC("B001"), () => new TestGate());
            var handle = world.NewDestructable(ObjectTypes.FourCC("B001"));
            var gate = Destructable.Of(handle)!;
            TestLog.Lines.Clear();
            Inbound.DestructableDied(handle);
            gate.Remove();
            ExpectLog(new string[] { "gate died", "part died" }, "the log");
        }

        /// <summary>A living destructable that is removed reacts to no death, even when the game fires one inside RemoveDestructable.</summary>
        public static void RemoveLiving(FakeWorld world)
        {
            world.DeathOnRemove = true;
            ObjectTypes.Destructable(ObjectTypes.FourCC("B001"), () => new TestGate());
            var handle = world.NewDestructable(ObjectTypes.FourCC("B001"));
            var gate = Destructable.Of(handle)!;
            TestLog.Lines.Clear();
            gate.Remove();
            ExpectLog(new string[0], "the log");
            Expect.True(gate.HasEnded, "the removed destructable has ended");
        }
    }
}
