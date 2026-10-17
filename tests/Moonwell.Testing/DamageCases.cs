using Moonwell.Hosting;

namespace Moonwell.Testing
{
    /// <summary>Cases that check damage: its order of reactions, the amount a reaction changes, a damage inside a damage, and removal mid-dispatch.</summary>
    public static class DamageCases
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

        /// <summary>Damage runs the source's type and components, then the target's, the game's, then the target's player and the source's player.</summary>
        public static void Order(FakeWorld world)
        {
            Arena();
            var striker = Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), slot: 0))!;
            var dummy = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h000"), slot: 1))!;
            TestLog.Lines.Clear();
            var amount = Inbound.Damaged(dummy.Handle, striker.Handle, 100f, true);
            ExpectLog(new string[] { "striker dealt", "striker comp dealt", "dummy damaged", "dummy comp damaged", "game damaged", "p1 damaged", "p0 damaged" }, "the log");
            Expect.Near(100f, amount, "the amount");
        }

        /// <summary>A target component that halves the damage makes the call return the halved amount.</summary>
        public static void ChangeAmount(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("h000"), () => new Dummy());
            var dummy = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h000"), slot: 1))!;
            dummy.Add(new Halver());
            Expect.Near(50f, Inbound.Damaged(dummy.Handle, null, 100f, true), "the amount after the halving");
        }

        /// <summary>With no reaction anywhere, damage between units the runtime does not hold returns the amount and makes no objects.</summary>
        public static void UnknownUnits(FakeWorld world)
        {
            var target = world.NewUnit(ObjectTypes.FourCC("ofoo"));
            var source = world.NewUnit(ObjectTypes.FourCC("ofoo"));
            Expect.Near(100f, Inbound.Damaged(target, source, 100f, true), "the amount");
            Expect.True(Units.Find(target) == null, "the target is not made");
            Expect.True(Units.Find(source) == null, "the source is not made");
        }

        /// <summary>The world is asked to deliver damage only once a class overrides OnDamaged.</summary>
        public static void Listen(FakeWorld world)
        {
            Expect.True(!world.Listening.Contains(GameEvent.Damage), "Damage is not listened for before a class needs it");
            ObjectTypes.Unit(ObjectTypes.FourCC("h000"), () => new Dummy());
            Expect.True(world.Listening.Contains(GameEvent.Damage), "Damage is listened for");
        }

        /// <summary>A damage dealt from inside OnDamaged runs fully inside the call, and the outer damage keeps its own amount.</summary>
        public static void Nested(FakeWorld world)
        {
            Arena();
            var striker = Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), slot: 0))!;
            var dummy = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h000"), slot: 1))!;
            dummy.Get<DummyComp>()!.Reply = striker.Handle;
            TestLog.Lines.Clear();
            var amount = Inbound.Damaged(dummy.Handle, striker.Handle, 100f, true);
            ExpectLog(new string[] { "striker dealt", "striker comp dealt", "dummy damaged", "dummy comp damaged", "game damaged", "p0 damaged", "p1 damaged", "game damaged", "p1 damaged", "p0 damaged" }, "the log");
            Expect.Near(100f, amount, "the outer amount");
        }

        /// <summary>A reaction that removes its own unit stops the unit's later reactions, nothing is reported, and the map's reactions still run.</summary>
        public static void ActorEndsMidDispatch(FakeWorld world)
        {
            Arena();
            var striker = Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), slot: 0))!;
            var dummy = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h000"), slot: 1))!;
            dummy.Add(new Remover());
            dummy.Add(new B());
            TestLog.Lines.Clear();
            var amount = Inbound.Damaged(dummy.Handle, striker.Handle, 100f, true);
            ExpectLog(new string[] { "striker dealt", "striker comp dealt", "dummy damaged", "dummy comp damaged", "game damaged", "p1 damaged", "p0 damaged" }, "the log");
            Expect.Equal(0, world.Logged.Count, "the lines logged");
            Expect.True(dummy.HasEnded, "the dummy has ended");
            Expect.Near(100f, amount, "the amount");
        }

        /// <summary>A reaction that throws is reported, and the reactions after it still run.</summary>
        public static void ErrorIsolated(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("h000"), () => new Dummy());
            var dummy = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h000"), slot: 1))!;
            dummy.Add(new A());
            dummy.Add(new B());
            TestLog.Lines.Clear();
            Inbound.Damaged(dummy.Handle, null, 100f, true);
            ExpectLog(new string[] { "dummy damaged", "dummy comp damaged", "b damaged" }, "the log");
            Expect.Equal(1, world.Logged.Count, "the lines logged");
            Expect.Equal("Moonwell: A.OnDamaged failed: boom", world.Logged[0], "the error line");
        }
    }
}
