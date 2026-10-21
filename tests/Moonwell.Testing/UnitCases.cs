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

        /// <summary>A unit's life, most life, mana and most mana are the game's: read and set, each apart.</summary>
        public static void LifeAndMana(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var unit = Unit.Of(handle)!;
            Expect.Near(0.0f, unit.Life, "the game's life before");

            unit.MaxLife = 420.0f;
            unit.Life = 300.5f;
            unit.MaxMana = 100.0f;
            unit.Mana = 40.0f;

            Expect.Near(300.5f, unit.Life, "the new life");
            Expect.Near(420.0f, unit.MaxLife, "the new most life");
            Expect.Near(40.0f, unit.Mana, "the new mana");
            Expect.Near(100.0f, unit.MaxMana, "the new most mana");
            Expect.Near(300.5f, world.LifeOf(handle), "the game's life");
            Expect.Near(420.0f, world.MaxLifeOf(handle), "the game's most life");
            Expect.Near(40.0f, world.ManaOf(handle), "the game's mana");
            Expect.Near(100.0f, world.MaxManaOf(handle), "the game's most mana");

            var other = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            Expect.Near(0.0f, other.Life, "another unit's life");
            Expect.Near(0.0f, other.Mana, "another unit's mana");
        }

        /// <summary>PlaceAt puts the unit at a place without the game's pathing, and X and Y read it.</summary>
        public static void Position(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var unit = Unit.Of(handle)!;
            Expect.Near(0.0f, unit.X, "x before");
            Expect.Near(0.0f, unit.Y, "y before");

            unit.PlaceAt(128.5f, -64.0f);

            Expect.Near(128.5f, unit.X, "the new x");
            Expect.Near(-64.0f, unit.Y, "the new y");
            Expect.Equal(1, world.UnitCalls.Count, "calls to the game");
            Expect.Equal("place", world.UnitCalls[0], "the call was a place");
            Expect.Near(128.5f, world.UnitX(handle), "the game's x");
            Expect.Near(-64.0f, world.UnitY(handle), "the game's y");
        }

        /// <summary>MoveTo moves the unit through the game's pathing, which is another call than PlaceAt.</summary>
        public static void MoveTo(FakeWorld world)
        {
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;

            unit.MoveTo(512.0f, 256.0f);

            Expect.Equal(1, world.UnitCalls.Count, "calls to the game");
            Expect.Equal("move", world.UnitCalls[0], "the call was a move");
            Expect.Near(512.0f, unit.X, "the new x");
            Expect.Near(256.0f, unit.Y, "the new y");
        }

        /// <summary>Setting Facing orders a turn at the turn rate, which does not change the facing at once; FaceNow does.</summary>
        public static void Facing(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var unit = Unit.Of(handle)!;
            unit.FaceNow(90.0f);
            Expect.Near(90.0f, unit.Facing, "the facing after FaceNow");

            unit.Facing = 180.0f;

            Expect.Near(90.0f, unit.Facing, "the facing is not turned at once");
            Expect.Near(180.0f, world.TurnTargets[handle], "the facing the unit was ordered to");
            Expect.Equal(2, world.UnitCalls.Count, "calls to the game");
            Expect.Equal("face-now", world.UnitCalls[0], "the first call");
            Expect.Equal("turn", world.UnitCalls[1], "the second call");

            unit.FaceNow(270.0f);

            Expect.Near(270.0f, unit.Facing, "the facing after the second FaceNow");
        }

        /// <summary>A unit the game has removed, found by the sweep or removed by Remove, gives the values it last read, ignores setters and actions, and the game is not asked.</summary>
        public static void GoneReadsLast(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var swept = Unit.Of(handle)!;
            SetAndRead(swept);
            var calls = world.UnitCalls.Count;

            world.VanishUnit(handle);
            world.Advance(0.3f);
            Expect.True(swept.IsGone, "the sweep found the unit gone");
            world.GoneCalls = 0;
            ExpectGoneIsQuiet(world, swept, calls, "swept");

            var removed = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            SetAndRead(removed);
            calls = world.UnitCalls.Count;
            removed.Remove();
            world.GoneCalls = 0;
            ExpectGoneIsQuiet(world, removed, calls, "removed");
        }

        /// <summary>Sets a unit's life, mana, place and facing and reads each back, so the unit has seen them.</summary>
        private static void SetAndRead(Unit unit)
        {
            unit.MaxLife = 500.0f;
            unit.Life = 450.0f;
            unit.MaxMana = 200.0f;
            unit.Mana = 150.0f;
            unit.PlaceAt(10.0f, 20.0f);
            unit.FaceNow(45.0f);
            Expect.Near(450.0f, unit.Life, "life read");
            Expect.Near(500.0f, unit.MaxLife, "most life read");
            Expect.Near(150.0f, unit.Mana, "mana read");
            Expect.Near(200.0f, unit.MaxMana, "most mana read");
            Expect.Near(10.0f, unit.X, "x read");
            Expect.Near(20.0f, unit.Y, "y read");
            Expect.Near(45.0f, unit.Facing, "facing read");
        }

        /// <summary>Checks that a gone unit gives the values SetAndRead left and that its setters and actions do not reach the game.</summary>
        private static void ExpectGoneIsQuiet(FakeWorld world, Unit unit, int calls, string what)
        {
            Expect.Near(450.0f, unit.Life, what + ": life");
            Expect.Near(500.0f, unit.MaxLife, what + ": most life");
            Expect.Near(150.0f, unit.Mana, what + ": mana");
            Expect.Near(200.0f, unit.MaxMana, what + ": most mana");
            Expect.Near(10.0f, unit.X, what + ": x");
            Expect.Near(20.0f, unit.Y, what + ": y");
            Expect.Near(45.0f, unit.Facing, what + ": facing");

            unit.Life = 1.0f;
            unit.MaxLife = 1.0f;
            unit.Mana = 1.0f;
            unit.MaxMana = 1.0f;
            unit.PlaceAt(99.0f, 99.0f);
            unit.MoveTo(99.0f, 99.0f);
            unit.Facing = 99.0f;
            unit.FaceNow(99.0f);

            Expect.Near(450.0f, unit.Life, what + ": life after a set");
            Expect.Near(10.0f, unit.X, what + ": x after PlaceAt");
            Expect.Near(45.0f, unit.Facing, what + ": facing after a set");
            Expect.Equal(0, world.GoneCalls, what + ": no call to the game for the gone unit");
            Expect.Equal(calls, world.UnitCalls.Count, what + ": no action reached the game");
        }

        /// <summary>A static dictionary keyed by the unit object loads, and finds a unit by its object.</summary>
        public static void DictionaryKey(FakeWorld world)
        {
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            KeyedNames.Names[unit] = "footman";
            Expect.Equal("footman", KeyedNames.Names[unit], "the unit's name");
            KeyedNames.Names.Remove(unit);
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
