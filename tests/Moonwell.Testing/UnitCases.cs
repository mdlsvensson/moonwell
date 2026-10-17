using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell.Testing
{
    /// <summary>Cases that check object ids, and units: their classes, their components, entering, removal and the sweep.</summary>
    public static class UnitCases
    {
        /// <summary>A unit that adds a Tally when it leaves.</summary>
        private sealed class AddingUnit : Unit
        {
            /// <summary>The Tally added when the unit leaves.</summary>
            public Tally? Added;

            /// <summary>Adds a Tally, which ends with the unit.</summary>
            protected override void OnLeave()
            {
                Added = Add(new Tally());
            }
        }

        /// <summary>A game component that keeps the last unit it was told entered.</summary>
        private sealed class EnterWatcher : GameComponent
        {
            /// <summary>The unit the last OnUnitEnter was given.</summary>
            public Unit? Seen;

            /// <summary>Keeps the unit.</summary>
            public override void OnUnitEnter(Unit unit)
            {
                Seen = unit;
            }
        }

        /// <summary>Checks that the log is exactly the expected lines, in order.</summary>
        private static void ExpectLog(string[] expected, string what)
        {
            Expect.Equal(expected.Length, TestLog.Lines.Count, what + ": line count");
            for (var i = 0; i < expected.Length; i++)
            {
                Expect.Equal(expected[i], TestLog.Lines[i], what + ": line " + i);
            }
        }

        /// <summary>Checks that the log ends with the expected lines, in order.</summary>
        private static void ExpectLogEnds(string[] expected, string what)
        {
            var start = TestLog.Lines.Count - expected.Length;
            Expect.True(start >= 0, what + ": enough lines");
            for (var i = 0; i < expected.Length; i++)
            {
                Expect.Equal(expected[i], TestLog.Lines[start + i], what + ": line " + i);
            }
        }

        /// <summary>FourCC packs four characters, the first one highest, into one int.</summary>
        public static void FourCC(FakeWorld world)
        {
            Expect.Equal(0x68666F6F, ObjectTypes.FourCC("hfoo"), "the id of hfoo");
            Expect.Equal(0x48303031, ObjectTypes.FourCC("H001"), "the id of H001");
        }

        /// <summary>Registering an id a second time throws.</summary>
        public static void Duplicate(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("H001"), () => new TestHero());
            Expect.Throws<MoonwellException>(() => ObjectTypes.Unit(ObjectTypes.FourCC("H001"), () => new TestHero()), "a second H001");
        }

        /// <summary>IdOf gives the id a class is registered with, and throws for a class never registered.</summary>
        public static void IdOf(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("H001"), () => new TestHero());
            Expect.Equal(ObjectTypes.FourCC("H001"), ObjectTypes.IdOf<TestHero>(), "the id of TestHero");
            Expect.Throws<MoonwellException>(() => ObjectTypes.IdOf<Unregistered>(), "the id of an unregistered class");
        }

        /// <summary>A unit of a registered type is an object of its class, with its type id, and Of gives the same object again.</summary>
        public static void RegisteredClass(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("H001"), () => new TestHero());
            var handle = world.NewUnit(ObjectTypes.FourCC("H001"), hero: true);
            var found = Unit.Of(handle);
            Expect.True(found is TestHero, "the unit is a TestHero");
            Expect.Equal(ObjectTypes.FourCC("H001"), found!.TypeId, "the type id");
            Expect.Same(found, Unit.Of(handle), "the unit asked for again");
        }

        /// <summary>An unregistered unit is a Unit, and an unregistered hero a Hero.</summary>
        public static void FallbackClass(FakeWorld world)
        {
            Expect.Equal("Unit", Names.NameOf(Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!), "an unregistered unit");
            Expect.Equal("Hero", Names.NameOf(Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo"), hero: true))!), "an unregistered hero");
        }

        /// <summary>Taking in a unit adds its starting components before OnEnter runs.</summary>
        public static void ComponentsBeforeEnter(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("H001"), () => new TestHero());
            TestLog.Lines.Clear();
            Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), hero: true));
            ExpectLog(new string[] { "tally added", "enter" }, "the log");
        }

        /// <summary>A unit that vanishes is ended by the sweep: OnLeave runs, its components end, and the game is not asked to remove it.</summary>
        public static void SweepDetectsRemoval(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("H001"), () => new TestHero());
            var handle = world.NewUnit(ObjectTypes.FourCC("H001"), hero: true);
            TestLog.Lines.Clear();
            var hero = Unit.Of(handle)!;
            world.VanishUnit(handle);
            world.Advance(0.3f);
            Expect.True(hero.HasEnded, "the vanished unit has ended");
            Expect.True(hero.IsGone, "the vanished unit is gone");
            ExpectLogEnds(new string[] { "leave", "tally removed" }, "the log");
            Expect.Equal(0, world.RemovedUnits.Count, "the units the game was asked to remove");
        }

        /// <summary>A unit the game has removed still reads its owner, which the game no longer gives.</summary>
        public static void PlayerAfterRemoval(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"), slot: 1);
            var unit = Unit.Of(handle)!;
            world.VanishUnit(handle);
            Expect.Equal(1, unit.Player.Slot, "the owner's slot before the sweep");
            world.Advance(0.3f);
            Expect.Equal(1, unit.Player.Slot, "the owner's slot after the sweep");
        }

        /// <summary>Remove ends the unit once, removes it from the game, and forgets it.</summary>
        public static void Remove(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var found = Unit.Of(handle)!;
            found.Remove();
            found.Remove();
            Expect.True(found.HasEnded, "the removed unit has ended");
            Expect.Equal(1, world.RemovedUnits.Count, "the units the game was asked to remove");
            Expect.Same(handle, world.RemovedUnits[0], "the handle removed");
            Expect.True(Unit.Of(handle) == null, "the unit is forgotten after removal");
        }

        /// <summary>A unit created for an owner has that owner, and ends with it.</summary>
        public static void OwnedByOwner(FakeWorld world)
        {
            var log = new List<string>();
            var owner = new TestOwner("owner", log);
            var made = Unit.Create(ObjectTypes.FourCC("hfoo"), Player.Get(0), 0f, 0f, 0f, owner);
            Expect.Same(owner, made.Owner, "the owner of the unit");
            owner.End();
            Expect.Equal(1, world.RemovedUnits.Count, "the units the game was asked to remove");
            Expect.Same(made.Handle, world.RemovedUnits[0], "the handle removed");
        }

        /// <summary>A unit created inside Run without an owner is not owned by the current owner, and outlives it.</summary>
        public static void NotOwnedByCurrent(FakeWorld world)
        {
            var log = new List<string>();
            var owner = new TestOwner("owner", log);
            Unit? made = null;
            owner.Run(() => { made = Unit.Create(ObjectTypes.FourCC("hfoo"), Player.Get(0), 0f, 0f, 0f); });
            var kept = made!;
            Expect.True(kept.Owner == null, "the unit has no owner");
            owner.End();
            Expect.True(!kept.HasEnded, "the unit outlives the owner");
        }

        /// <summary>Create of a class gives an object of that class, made from its registered id.</summary>
        public static void CreateGeneric(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("H001"), () => new TestHero());
            var made = Unit.Create<TestHero>(Player.Get(0), 0f, 0f, 0f);
            Expect.Equal(ObjectTypes.FourCC("H001"), made.TypeId, "the type id");
        }

        /// <summary>When two units' OnLeave throw, the sweep still ends both, and the error is logged.</summary>
        public static void SweepContinuesAfterError(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("h002"), () => new ThrowingLeave());
            var first = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h002")))!;
            var second = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h002")))!;
            world.VanishUnit(first.Handle);
            world.VanishUnit(second.Handle);
            world.Advance(0.3f);
            Expect.True(first.HasEnded, "the first unit has ended");
            Expect.True(second.HasEnded, "the second unit has ended");
            Expect.True(world.Logged.Contains("Moonwell: ThrowingLeave.OnLeave failed: boom"), "the logged error");
        }

        /// <summary>A component added inside OnLeave is added, then ends with its unit, and leaves nothing behind.</summary>
        public static void AddInOnLeave(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("h003"), () => new AddingUnit());
            var made = (AddingUnit)Unit.Of(world.NewUnit(ObjectTypes.FourCC("h003")))!;
            TestLog.Lines.Clear();
            made.Remove();
            Expect.True(made.Added != null && made.Added.HasEnded, "the tally added in OnLeave has ended");
            ExpectLog(new string[] { "tally added", "tally removed" }, "the log");
        }

        /// <summary>A unit entering is taken in when its type is registered, or once a game component wants OnUnitEnter.</summary>
        public static void Enter(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("H001"), () => new TestHero());

            var registered = world.NewUnit(ObjectTypes.FourCC("H001"), hero: true);
            Inbound.Entered(registered);
            Expect.True(Units.Find(registered) != null, "a registered unit is taken in");

            var plain = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            Inbound.Entered(plain);
            Expect.True(Units.Find(plain) == null, "an unregistered unit is not taken in");

            var watcher = Game.Instance.Add(new EnterWatcher());
            var later = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            Inbound.Entered(later);
            Expect.True(Units.Find(later) != null, "the unit is taken in once a component wants it");
            Expect.Same(Units.Find(later), watcher.Seen, "the unit given to OnUnitEnter");
        }
    }
}
