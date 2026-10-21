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

        /// <summary>A unit's level is the game's: a hero's hero level.</summary>
        public static void Level(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("Hpal"), hero: true);
            var unit = Unit.Of(handle)!;
            Expect.Equal(1, unit.Level, "the level before");

            world.SetLevel(handle, 7);

            Expect.Equal(7, unit.Level, "the level the game gives");
            Expect.Equal(1, Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!.Level, "another unit's level");
        }

        /// <summary>Kill goes through the game; the death comes back as the game's own event, once, with no killer, and the unit is no longer alive.</summary>
        public static void Kill(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("h000"), () => new Dummy());
            var dummy = (Dummy)Unit.Of(world.NewUnit(ObjectTypes.FourCC("h000")))!;
            TestLog.Lines.Clear();
            Expect.True(dummy.IsAlive, "alive before");

            dummy.Kill();

            Expect.Equal(1, world.KillCalls, "calls to the game's KillUnit");
            Expect.Equal(2, TestLog.Lines.Count, "reactions to the death");
            Expect.Equal("dummy died", TestLog.Lines[0], "the unit's reaction, once");
            Expect.Equal("dummy comp died", TestLog.Lines[1], "the component's reaction, once");
            Expect.True(dummy.LastKiller == null, "the killer given to OnDeath");
            Expect.True(!dummy.IsAlive, "alive after");
            Expect.True(!dummy.HasEnded, "a death is not an end");
        }

        /// <summary>A second Kill on a dead unit fires no death event (seen 10-21): the unit's reaction runs once.</summary>
        public static void KillTwice(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("h000"), () => new Dummy());
            var dummy = (Dummy)Unit.Of(world.NewUnit(ObjectTypes.FourCC("h000")))!;
            TestLog.Lines.Clear();

            dummy.Kill();
            dummy.Kill();

            Expect.Equal(2, world.KillCalls, "calls to the game's KillUnit");
            Expect.Equal(2, TestLog.Lines.Count, "reactions to the death, once for the unit and its component");
            Expect.True(!dummy.IsAlive, "alive after");
        }

        /// <summary>After a Kill the life is 0 (seen 10-21), also the last life a unit gives once the game has removed it, whatever was read before.</summary>
        public static void KillLeavesNoLife(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var unit = Unit.Of(handle)!;
            unit.Life = 100.0f;
            Expect.Near(100.0f, unit.Life, "life before");

            unit.Kill();
            world.VanishUnit(handle);
            world.Advance(0.3f);

            Expect.True(unit.IsGone, "the sweep found the unit gone");
            Expect.Near(0.0f, unit.Life, "the life a killed then removed unit gives");
        }

        /// <summary>A unit the game has removed is not alive, and Kill does not reach the game.</summary>
        public static void KillGone(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("h000"), () => new Dummy());
            var handle = world.NewUnit(ObjectTypes.FourCC("h000"));
            var dummy = (Dummy)Unit.Of(handle)!;
            TestLog.Lines.Clear();
            world.VanishUnit(handle);
            world.Advance(0.3f);
            Expect.True(dummy.IsGone, "the sweep found the unit gone");
            world.GoneCalls = 0;

            dummy.Kill();

            Expect.True(!dummy.IsAlive, "a gone unit is not alive");
            Expect.Equal(0, world.KillCalls, "calls to the game's KillUnit");
            Expect.Equal(0, world.GoneCalls, "no call to the game for the gone unit");
            Expect.Equal(0, TestLog.Lines.Count, "no reaction");
        }

        /// <summary>IsPaused pauses and unpauses the unit in the game, and reads the game.</summary>
        public static void Pause(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var unit = Unit.Of(handle)!;
            Expect.True(!unit.IsPaused, "not paused before");

            unit.IsPaused = true;

            Expect.True(unit.IsPaused, "paused");
            Expect.True(world.PausedUnits.Contains(handle), "the game has the unit paused");

            unit.IsPaused = false;

            Expect.True(!unit.IsPaused, "unpaused");
            Expect.True(!world.PausedUnits.Contains(handle), "the game has the unit unpaused");
        }

        /// <summary>Scale sets the size of the unit's model in the game, and reads back.</summary>
        public static void Scale(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var unit = Unit.Of(handle)!;
            Expect.Near(1.0f, unit.Scale, "the scale before");

            unit.Scale = 1.75f;

            Expect.Near(1.75f, unit.Scale, "the new scale");
            Expect.Equal(1, world.ScaleCalls, "calls to the game");
            Expect.Near(1.75f, world.ScaleOf(handle), "the game's scale");
        }

        /// <summary>Tint hands the four values to the game; the alpha is opaque when left out.</summary>
        public static void Tint(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var unit = Unit.Of(handle)!;

            unit.Tint(255, 128, 0, 64);

            Expect.Equal(1, world.Tints.Count, "units tinted");
            Expect.Equal(4, world.Tints[handle].Length, "values");
            Expect.Equal(255, world.Tints[handle][0], "red");
            Expect.Equal(128, world.Tints[handle][1], "green");
            Expect.Equal(0, world.Tints[handle][2], "blue");
            Expect.Equal(64, world.Tints[handle][3], "alpha");

            unit.Tint(10, 20, 30);

            Expect.Equal(10, world.Tints[handle][0], "red, second call");
            Expect.Equal(20, world.Tints[handle][1], "green, second call");
            Expect.Equal(30, world.Tints[handle][2], "blue, second call");
            Expect.Equal(255, world.Tints[handle][3], "alpha defaults to opaque");
        }

        /// <summary>FlyHeight is set in the game and read from it.</summary>
        public static void FlyHeight(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var unit = Unit.Of(handle)!;
            Expect.Near(0.0f, unit.FlyHeight, "the height before");

            unit.FlyHeight = 250.5f;

            Expect.Near(250.5f, unit.FlyHeight, "the new height");
            Expect.Near(250.5f, world.FlyHeightOf(handle), "the game's height");
        }

        /// <summary>Order and Stop give the game an order string with no place and no target, and return the game's answer.</summary>
        public static void OrderImmediate(FakeWorld world)
        {
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;

            Expect.True(unit.Order("holdposition"), "the game accepts the order");
            Expect.Equal(1, world.Orders.Count, "orders given");
            Expect.Equal("immediate holdposition", world.Orders[0], "the order as given");

            world.OrderAnswer = false;
            Expect.True(!unit.Order("stop"), "the game's refusal is returned");

            world.OrderAnswer = true;
            unit.Stop();
            Expect.Equal(3, world.Orders.Count, "orders given after Stop");
            Expect.Equal("immediate stop", world.Orders[2], "Stop is the stop order");
        }

        /// <summary>OrderAt gives the game an order string and a place, and returns the game's answer.</summary>
        public static void OrderPoint(FakeWorld world)
        {
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;

            Expect.True(unit.OrderAt("move", 100.5f, -200.0f), "the game accepts the order");
            Expect.Equal(1, world.Orders.Count, "orders given");
            Expect.Equal("point move", world.Orders[0], "the order as given");
            Expect.Near(100.5f, world.OrderPoints[0], "x");
            Expect.Near(-200.0f, world.OrderPoints[1], "y");

            world.OrderAnswer = false;
            Expect.True(!unit.OrderAt("attack", 1.0f, 2.0f), "the game's refusal is returned");
            Expect.Equal("point attack", world.Orders[1], "the second order as given");
        }

        /// <summary>OrderOn gives the game the widget of a unit, an item or a destructable, and returns the game's answer.</summary>
        public static void OrderTarget(FakeWorld world)
        {
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var targetHandle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var target = Unit.Of(targetHandle)!;
            var itemHandle = world.NewItem(ObjectTypes.FourCC("ofoo"));
            var item = Item.Of(itemHandle)!;
            var treeHandle = world.NewDestructable(ObjectTypes.FourCC("LTlt"));
            var tree = Destructable.Of(treeHandle)!;

            Expect.True(unit.OrderOn("attack", target), "the game accepts the order on a unit");
            Expect.True(unit.OrderOn("smart", item), "the game accepts the order on an item");
            Expect.True(unit.OrderOn("harvest", tree), "the game accepts the order on a destructable");

            Expect.Equal(3, world.Orders.Count, "orders given");
            Expect.Equal("target attack", world.Orders[0], "the order on the unit");
            Expect.Equal("target smart", world.Orders[1], "the order on the item");
            Expect.Equal("target harvest", world.Orders[2], "the order on the destructable");
            Expect.Same(targetHandle, world.OrderTargets[0], "the unit's handle reached the game");
            Expect.Same(itemHandle, world.OrderTargets[1], "the item's handle reached the game");
            Expect.Same(treeHandle, world.OrderTargets[2], "the destructable's handle reached the game");

            world.OrderAnswer = false;
            Expect.True(!unit.OrderOn("attack", target), "the game's refusal is returned");
        }

        /// <summary>A removed unit takes no order and a removed target is not ordered on: false, and the game is not called.</summary>
        public static void OrderGone(FakeWorld world)
        {
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var targetHandle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var target = Unit.Of(targetHandle)!;
            var itemHandle = world.NewItem(ObjectTypes.FourCC("ofoo"));
            var item = Item.Of(itemHandle)!;
            var other = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;

            world.VanishUnit(targetHandle);
            world.VanishItem(itemHandle);
            world.Advance(0.3f);
            Expect.True(target.IsGone && item.IsGone, "the sweep found the target and the item gone");
            world.GoneCalls = 0;

            Expect.True(!other.OrderOn("attack", target), "no order on a removed unit");
            Expect.True(!other.OrderOn("smart", item), "no order on a removed item");
            Expect.Equal(0, world.Orders.Count, "no order reached the game for a removed target");

            other.Remove();
            world.GoneCalls = 0;

            Expect.True(!other.Order("stop"), "a removed unit takes no immediate order");
            Expect.True(!other.OrderAt("move", 1.0f, 2.0f), "a removed unit takes no point order");
            Expect.True(!other.OrderOn("attack", unit), "a removed unit takes no target order");
            other.Stop();
            Expect.Equal(0, world.Orders.Count, "no order reached the game from a removed unit");
            Expect.Equal(0, world.GoneCalls, "no call to the game for the gone unit");
        }

        /// <summary>A unit the game has removed, found by the sweep or removed by Remove, gives the values it last read, ignores setters and actions, and the game is not asked.</summary>
        public static void GoneReadsLast(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var swept = Unit.Of(handle)!;
            SetAndRead(world, swept);
            var calls = world.UnitCalls.Count;

            world.VanishUnit(handle);
            world.Advance(0.3f);
            Expect.True(swept.IsGone, "the sweep found the unit gone");
            world.GoneCalls = 0;
            ExpectGoneIsQuiet(world, swept, calls, "swept");

            var removed = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            SetAndRead(world, removed);
            calls = world.UnitCalls.Count;
            removed.Remove();
            world.GoneCalls = 0;
            ExpectGoneIsQuiet(world, removed, calls, "removed");

            // Values set and never read: the setters keep them, so they are what a gone unit gives.
            var unread = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            unread.MaxLife = 500.0f;
            unread.Life = 450.0f;
            unread.MaxMana = 200.0f;
            unread.Mana = 150.0f;
            unread.Facing = 45.0f;
            unread.IsPaused = true;
            unread.Scale = 2.5f;
            unread.FlyHeight = 120.0f;
            unread.Remove();
            Expect.Near(450.0f, unread.Life, "unread: life");
            Expect.Near(500.0f, unread.MaxLife, "unread: most life");
            Expect.Near(150.0f, unread.Mana, "unread: mana");
            Expect.Near(200.0f, unread.MaxMana, "unread: most mana");
            Expect.Near(45.0f, unread.Facing, "unread: facing");
            Expect.True(unread.IsPaused, "unread: paused");
            Expect.Near(2.5f, unread.Scale, "unread: scale");
            Expect.Near(120.0f, unread.FlyHeight, "unread: fly height");
        }

        /// <summary>Sets a unit's life, mana, place, facing, pause, scale and fly height and reads each back, so the unit has seen them; the game gives it level 3.</summary>
        private static void SetAndRead(FakeWorld world, Unit unit)
        {
            world.SetLevel(unit.Handle, 3);
            unit.IsPaused = true;
            unit.Scale = 2.5f;
            unit.FlyHeight = 120.0f;
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
            Expect.True(unit.IsPaused, "paused read");
            Expect.Near(2.5f, unit.Scale, "scale read");
            Expect.Near(120.0f, unit.FlyHeight, "fly height read");
            Expect.Equal(3, unit.Level, "level read");
            Expect.True(unit.IsAlive, "alive read");
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
            Expect.True(unit.IsPaused, what + ": paused");
            Expect.Near(2.5f, unit.Scale, what + ": scale");
            Expect.Near(120.0f, unit.FlyHeight, what + ": fly height");
            Expect.Equal(3, unit.Level, what + ": level");
            Expect.True(!unit.IsAlive, what + ": a gone unit is not alive");

            unit.Life = 1.0f;
            unit.MaxLife = 1.0f;
            unit.Mana = 1.0f;
            unit.MaxMana = 1.0f;
            unit.PlaceAt(99.0f, 99.0f);
            unit.MoveTo(99.0f, 99.0f);
            unit.Facing = 99.0f;
            unit.FaceNow(99.0f);
            unit.IsPaused = false;
            unit.Scale = 9.0f;
            unit.FlyHeight = 9.0f;
            unit.Tint(1, 2, 3, 4);
            unit.Kill();

            Expect.Near(450.0f, unit.Life, what + ": life after a set");
            Expect.True(unit.IsPaused, what + ": paused after a set");
            Expect.Near(2.5f, unit.Scale, what + ": scale after a set");
            Expect.Near(120.0f, unit.FlyHeight, what + ": fly height after a set");
            Expect.Equal(0, world.Tints.Count, what + ": no tint reached the game");
            Expect.Equal(0, world.KillCalls, what + ": no kill reached the game");
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

        /// <summary>A ground unit's fly height shows only after Amrf is added and removed (seen 10-21): FlyHeight does that once per unit, the first time it sets, and one native call per set after; a unit that flies gets no trick.</summary>
        public static void FlyHeightGroundUnit(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var unit = Unit.Of(handle)!;
            var amrf = FakeWorld.CrowForm;

            unit.FlyHeight = 150.0f;

            Expect.True(world.Floats(handle), "the ground unit floats after the first set");
            Expect.Equal(2, world.AbilityCalls.Count, "ability calls for the trick");
            Expect.Equal("add " + amrf, world.AbilityCalls[0], "the trick adds Amrf");
            Expect.Equal("remove " + amrf, world.AbilityCalls[1], "the trick removes Amrf");
            Expect.Near(150.0f, unit.FlyHeight, "the height");

            unit.FlyHeight = 300.0f;
            unit.FlyHeight = 0.0f;
            unit.FlyHeight = 80.0f;

            Expect.Equal(2, world.AbilityCalls.Count, "no second trick");
            Expect.True(world.Floats(handle), "still floating");
            Expect.Near(80.0f, unit.FlyHeight, "the last height");

            var other = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            other.FlyHeight = 10.0f;
            Expect.Equal(4, world.AbilityCalls.Count, "another unit gets its own trick");

            var birdHandle = world.NewUnit(ObjectTypes.FourCC("hgry"));
            world.MakeFlying(birdHandle);
            var bird = Unit.Of(birdHandle)!;
            bird.FlyHeight = 200.0f;
            Expect.Equal(4, world.AbilityCalls.Count, "a unit that flies gets no trick");
            Expect.True(world.Floats(birdHandle), "the flyer floats");
        }

        /// <summary>A unit's most life and most mana are whole numbers the game clamps the life and mana down to (BlzSetUnitMaxHP, seen 10-21).</summary>
        public static void MaxClampsLifeAndMana(FakeWorld world)
        {
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            unit.MaxLife = 500.0f;
            unit.Life = 450.0f;
            unit.MaxMana = 200.0f;
            unit.Mana = 150.0f;

            unit.MaxLife = 400.0f;
            unit.MaxMana = 300.0f;

            Expect.Near(400.0f, unit.MaxLife, "most life");
            Expect.Near(400.0f, unit.Life, "life clamped down to the new most");
            Expect.Near(300.0f, unit.MaxMana, "most mana");
            Expect.Near(150.0f, unit.Mana, "mana kept when the most rises");

            unit.MaxLife = 400.7f;
            unit.MaxMana = 300.9f;

            Expect.Near(400.0f, unit.MaxLife, "most life truncated to a whole number");
            Expect.Near(300.0f, unit.MaxMana, "most mana truncated to a whole number");
        }

        /// <summary>What a setter stored is what a unit gives after the game removed it, unread: the whole number the game keeps for the most, and a life or mana clamped to it.</summary>
        public static void MaxLastValues(FakeWorld world)
        {
            var first = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var unit = Unit.Of(first)!;
            unit.MaxLife = 400.7f;
            unit.MaxMana = 90.9f;
            unit.Life = 450.0f;
            unit.Mana = 100.0f;
            world.VanishUnit(first);
            Expect.Near(400.0f, unit.MaxLife, "most life: the whole number");
            Expect.Near(90.0f, unit.MaxMana, "most mana: the whole number");
            Expect.Near(400.0f, unit.Life, "life: clamped to the most");
            Expect.Near(90.0f, unit.Mana, "mana: clamped to the most");

            var second = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var lowered = Unit.Of(second)!;
            lowered.MaxLife = 500.0f;
            lowered.Life = 450.0f;
            lowered.MaxMana = 200.0f;
            lowered.Mana = 150.0f;
            lowered.MaxLife = 300.5f;
            lowered.MaxMana = 100.0f;
            world.VanishUnit(second);
            Expect.Near(300.0f, lowered.Life, "life: falls to the lowered most");
            Expect.Near(100.0f, lowered.Mana, "mana: falls to the lowered most");

            var third = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var manaless = Unit.Of(third)!;
            manaless.MaxMana = 0.0f;
            manaless.Mana = -5.0f;
            world.VanishUnit(third);
            Expect.Near(0.0f, manaless.Mana, "mana of a unit without mana: 0, not negative");
        }

        /// <summary>A unit that already has Amrf gets no add and remove (the remove would take its own ability away), and floats at once.</summary>
        public static void FlyHeightWithAmrf(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            world.SetAbilityLevel(handle, FakeWorld.CrowForm, 1);
            var unit = Unit.Of(handle)!;

            unit.FlyHeight = 150.0f;

            Expect.Equal(0, world.AbilityCalls.Count, "no ability call");
            Expect.True(world.Floats(handle), "the unit floats");
            Expect.Near(150.0f, unit.FlyHeight, "the height");
        }

        /// <summary>A removed unit reads false for IsPaused before the sweep (every getter gives 0, seen 10-21): the last value stays.</summary>
        public static void PausedGoneBeforeSweep(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var unit = Unit.Of(handle)!;
            unit.IsPaused = true;
            Expect.True(unit.IsPaused, "paused read");

            world.VanishUnit(handle);

            Expect.True(unit.IsPaused, "paused before the sweep");
            world.Advance(0.3f);
            Expect.True(unit.IsGone, "the sweep found the unit gone");
            Expect.True(unit.IsPaused, "paused after the sweep");

            var live = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            live.IsPaused = true;
            Expect.True(live.IsPaused, "a live unit reads true");
            live.IsPaused = false;
            Expect.True(!live.IsPaused, "a live unit reads false");
        }

        /// <summary>The game removes a unit and every getter reads 0 before the sweep finds it (seen 10-21): the last values stay, and a real 0 of a live unit is still read.</summary>
        public static void GoneBeforeSweep(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var unit = Unit.Of(handle)!;
            SetAndRead(world, unit);
            Expect.Equal(6, unit.InventorySize, "inventory size read");

            world.VanishUnit(handle);

            Expect.True(!unit.IsGone, "the sweep has not run");
            ExpectLastValues(unit, "before the sweep");
            world.Advance(0.3f);
            Expect.True(unit.IsGone, "the sweep found the unit gone");
            ExpectLastValues(unit, "after the sweep");

            // A unit never read keeps nothing to give back, and the zero it reads is the last value.
            var unread = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            world.VanishUnit(unread.Handle);
            Expect.Near(0.0f, unread.Life, "unread: life");
            Expect.Near(1.0f, unread.Scale, "unread: scale stays the default, not 0");
            Expect.True(!unread.IsGone, "unread: still not swept");

            // A live unit whose value really is 0 is read as 0.
            var live = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            live.PlaceAt(10.0f, 20.0f);
            live.Life = 50.0f;
            Expect.Near(10.0f, live.X, "live: x read");
            Expect.Near(50.0f, live.Life, "live: life read");
            live.PlaceAt(0.0f, 0.0f);
            live.Life = 0.0f;
            Expect.Near(0.0f, live.X, "live: x is 0");
            Expect.Near(0.0f, live.Y, "live: y is 0");
            Expect.Near(0.0f, live.Life, "live: life is 0");
        }

        /// <summary>A read inside Local.For of a unit the game removed gives the last value as well, and leaves it as it was.</summary>
        public static void GoneBeforeSweepInLocal(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var unit = Unit.Of(handle)!;
            unit.PlaceAt(10.0f, 20.0f);
            Expect.Near(10.0f, unit.X, "x read");
            world.VanishUnit(handle);

            var inside = 0.0f;
            Local.For(Player.Get(0), () => inside = unit.X);

            Expect.Near(10.0f, inside, "inside Local.For");
            Expect.Near(10.0f, unit.X, "outside");

            var other = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var lively = Unit.Of(other)!;
            lively.Life = 50.0f;
            lively.Scale = 2.5f;
            Expect.Near(50.0f, lively.Life, "life read");
            var untouched = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            world.VanishUnit(other);
            world.VanishUnit(untouched.Handle);

            var life = 0.0f;
            var scale = 0.0f;
            var plain = 0.0f;
            Local.For(Player.Get(0), () =>
            {
                life = lively.Life;
                scale = lively.Scale;
                plain = untouched.Scale;
            });

            Expect.Near(50.0f, life, "life inside Local.For");
            Expect.Near(2.5f, scale, "scale inside Local.For");
            Expect.Near(1.0f, plain, "an unset scale inside Local.For is the default, not 0");
            Expect.Near(50.0f, lively.Life, "life outside");
            Expect.Near(1.0f, untouched.Scale, "an unset scale outside");
        }

        /// <summary>The values SetAndRead left, as a unit the game removed gives them.</summary>
        private static void ExpectLastValues(Unit unit, string what)
        {
            Expect.Near(450.0f, unit.Life, what + ": life");
            Expect.Near(500.0f, unit.MaxLife, what + ": most life");
            Expect.Near(150.0f, unit.Mana, what + ": mana");
            Expect.Near(200.0f, unit.MaxMana, what + ": most mana");
            Expect.Near(10.0f, unit.X, what + ": x");
            Expect.Near(20.0f, unit.Y, what + ": y");
            Expect.Near(45.0f, unit.Facing, what + ": facing");
            Expect.Near(2.5f, unit.Scale, what + ": scale");
            Expect.Near(120.0f, unit.FlyHeight, what + ": fly height");
            Expect.Equal(3, unit.Level, what + ": level");
            Expect.Equal(6, unit.InventorySize, what + ": inventory size");
        }
    }
}
