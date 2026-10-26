namespace Moonwell.Testing
{
    /// <summary>Cases that check death: its order of reactions, a unit that kills itself, and that a death ends nothing.</summary>
    public static class DeathCases
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

        /// <summary>Registers the striker and the dummy, a game component and a player component for each of the two players.</summary>
        private static void Arena()
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("H001"), () => new Striker());
            ObjectTypes.Unit(ObjectTypes.FourCC("h000"), () => new Dummy());
            Game.Instance.Add(new GameWatch());
            Player.Get(0).Add(new PlayerWatch("p0"));
            Player.Get(1).Add(new PlayerWatch("p1"));
        }

        /// <summary>A death runs the dying unit's type and components, then the killer's, then the game's, then the dying unit's player and the killer's.</summary>
        public static void Order(FakeWorld world)
        {
            Arena();
            var striker = Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), slot: 0))!;
            var dummy = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h000"), slot: 1))!;
            TestLog.Lines.Clear();
            Inbound.Died(dummy.Handle, striker.Handle);
            ExpectLog(new string[] { "dummy died", "dummy comp died", "striker killed", "striker comp killed", "game died", "p1 died", "p0 died" }, "the log");
        }

        /// <summary>A death of a unit killed by another unit of its player runs that player's reactions once.</summary>
        public static void SamePlayer(FakeWorld world)
        {
            Arena();
            var striker = Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), slot: 1))!;
            var dummy = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h000"), slot: 1))!;
            TestLog.Lines.Clear();
            Inbound.Died(dummy.Handle, striker.Handle);
            ExpectLog(new string[] { "dummy died", "dummy comp died", "striker killed", "striker comp killed", "game died", "p1 died" }, "the log");
        }

        /// <summary>A unit that kills itself gives its OnDeath no killer, and runs no OnKill.</summary>
        public static void Self(FakeWorld world)
        {
            Arena();
            var dummy = (Dummy)Unit.Of(world.NewUnit(ObjectTypes.FourCC("h000"), slot: 1))!;
            TestLog.Lines.Clear();
            Inbound.Died(dummy.Handle, dummy.Handle);
            ExpectLog(new string[] { "dummy died", "dummy comp died", "game died", "p1 died" }, "the log");
            Expect.True(dummy.LastKiller == null, "the killer given to OnDeath");
        }

        /// <summary>A death is not an end: the unit has not ended and still has its component.</summary>
        public static void NotAnEnd(FakeWorld world)
        {
            Arena();
            var striker = Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), slot: 0))!;
            var dummy = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h000"), slot: 1))!;
            Inbound.Died(dummy.Handle, striker.Handle);
            Expect.True(!dummy.HasEnded, "the dummy has not ended");
            Expect.Equal(1, dummy.ComponentCount, "the components of the dummy");
        }
    }
}
