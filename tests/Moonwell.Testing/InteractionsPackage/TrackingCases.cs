using System;
using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell.Testing
{
    /// <summary>Cases for what the package tracks, on the default settings but cells of 256: a step of 1/32 second, a cell check of 0.1 second over a 2,048 by 2,048 world from (-1024, -1024).</summary>
    public static class TrackingCases
    {
        /// <summary>Installs a fake interactions world with cells of 256, which the expected cells of these cases count in (the default is 128), and returns it.</summary>
        private static FakeInteractionWorld Fresh()
        {
            var fake = new FakeInteractionWorld();
            Interactions.Install(fake);
            Interactions.CellSize = 256.0f;
            return fake;
        }

        /// <summary>Runs a case as the game's first use finds the package: not installed, nothing tracked or asked for, and a fake world standing in for the game's own, which the first use installs the package on. The game's world is the natives again after.</summary>
        private static void FirstUse(FakeInteractionWorld fake, Action run)
        {
            Tracking.Reset();
            Interactions.native = fake;
            try
            {
                run();
            }
            finally
            {
                Interactions.native = null;
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

        /// <summary>Makes a unit of the game at a place and takes it in.</summary>
        private static Unit UnitAt(FakeWorld world, float x, float y)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            world.MoveUnit(handle, x, y);
            return Unit.Of(handle)!;
        }

        /// <summary>Makes an item of the game at a place and takes it in.</summary>
        private static Item ItemAt(FakeWorld world, float x, float y)
        {
            var handle = world.NewItem(ObjectTypes.FourCC("ofoo"));
            world.MoveItem(handle, x, y);
            return Item.Of(handle)!;
        }

        /// <summary>The entry of a thing the case knows is tracked.</summary>
        private static Entry Tracked(Actor thing)
        {
            var entry = Tracking.EntryOf(thing);
            Expect.True(entry != null, "the thing is tracked");
            return entry!;
        }

        /// <summary>A map that only has gizmos tracks no unit and no item: nothing of the game is read, however long the cycle runs, and asking for units then tracks the ones there are.</summary>
        public static void OnlyWhenAsked(FakeWorld world)
        {
            var unit = UnitAt(world, 10.0f, 10.0f);
            var item = ItemAt(world, 20.0f, 20.0f);
            var fake = Fresh();
            var later = UnitAt(world, 30.0f, 30.0f);
            var well = Gizmo.Create(new Well(0.1f), 0.0f, 0.0f, Game.Instance);
            Steps(world, 40);
            Expect.Equal(0, fake.Reads, "no unit, item or destructable was read");
            Expect.True(Tracking.EntryOf(unit) == null, "a unit from before is not tracked");
            Expect.True(Tracking.EntryOf(later) == null, "a unit made after is not tracked");
            Expect.True(Tracking.EntryOf(item) == null, "an item is not tracked");
            Expect.True(Tracking.EntryOf(well) != null, "the gizmo is");
            Expect.Equal(1, Tracking.Count, "one thing is tracked");
            Tracking.Need(Tracking.UnitKind);
            Expect.True(Tracking.EntryOf(unit) != null, "asking for units tracks the unit from before");
            Expect.True(Tracking.EntryOf(later) != null, "and the one made after");
            Expect.True(Tracking.EntryOf(item) == null, "an item still is not");
        }

        /// <summary>Asking for units adds the live ones, each in the cell of its place, once; a unit made later comes in through the hook, and asking again changes nothing.</summary>
        public static void UnitsFilled(FakeWorld world)
        {
            var near = UnitAt(world, 10.0f, 10.0f);
            var right = UnitAt(world, 300.0f, 10.0f);
            Fresh();
            Tracking.Need(Tracking.UnitKind);
            var first = Tracked(near);
            var second = Tracked(right);
            Expect.Equal(36, first.Cell, "the cell of the unit at (10, 10)");
            Expect.Equal(37, second.Cell, "the cell of the unit at (300, 10)");
            Expect.Near(300.0f, second.X, "its X");
            Expect.True(first.HasPlace, "it has a place");
            Expect.Same(near, first.Thing, "the entry stands for the unit");
            Expect.Equal(Tracking.UnitKind, first.Kind, "of the unit kind");
            Expect.True(Tracking.Running, "the cycle runs");
            var made = UnitAt(world, -500.0f, -500.0f);
            var entry = Tracked(made);
            Expect.Equal(2 * 8 + 2, entry.Cell, "a unit made later is in the grid at once");
            Expect.Equal(3, Tracking.Count, "three things are tracked");
            Tracking.Need(Tracking.UnitKind);
            Expect.Equal(3, Tracking.Count, "asking again adds nothing");
            Expect.Same(first, Tracking.EntryOf(near), "and keeps the entries");
        }

        /// <summary>A class asks for the kinds it covers: a map's gizmo class only gizmos, Unit units, Actor all four.</summary>
        public static void KindsFromTypes(FakeWorld world)
        {
            var unit = UnitAt(world, 0.0f, 0.0f);
            var item = ItemAt(world, 0.0f, 0.0f);
            world.PlacedItems.Add(item.Handle);
            var handle = world.NewDestructable(ObjectTypes.FourCC("dfoo"));
            world.PlacedDestructables.Add(handle);
            Fresh();
            Tracking.NeedFor(typeof(Well));
            Expect.True(Tracking.EntryOf(unit) == null, "a gizmo class tracks no unit");
            Tracking.NeedFor(typeof(Unit));
            Expect.True(Tracking.EntryOf(unit) != null, "Unit tracks units");
            Expect.True(Tracking.EntryOf(item) == null, "and not items");
            Tracking.NeedFor(typeof(TestHero));
            Expect.Equal(1, Tracking.Count, "a subclass of Unit asks for units again");
            Tracking.NeedFor(typeof(Actor));
            Expect.True(Tracking.EntryOf(item) != null, "Actor tracks items");
            Expect.True(Tracking.EntryOf(Destructable.Of(handle)!) != null, "and destructables");
            Expect.Equal(3, Tracking.Count, "the unit, the item and the destructable");
        }

        /// <summary>A class with no place cannot be asked for, in a development build.</summary>
        public static void NoPlaceClass(FakeWorld world)
        {
            Fresh();
#if DEBUG
            Expect.Throws<MoonwellException>(() => Tracking.NeedFor(typeof(Player)), "Player has no place");
            Expect.Throws<MoonwellException>(() => Tracking.NeedFor(typeof(Game)), "the game has no place");
#endif
            Expect.Equal(0, Tracking.Count, "nothing is tracked");
        }

        /// <summary>40 units with a cell check of 0.1 and a step of 1/32 are read 13 a step, going round the list, so each is read again within 4 steps.</summary>
        public static void CellCheckSpread(FakeWorld world)
        {
            var fake = Fresh();
            var units = new List<Unit>();
            for (var i = 0; i < 40; i++)
            {
                units.Add(UnitAt(world, 0.0f, 0.0f));
            }

            Tracking.Need(Tracking.UnitKind);
            for (var i = 0; i < 40; i++)
            {
                world.MoveUnit(units[i].Handle, 100.0f, 0.0f);
            }

            fake.Reads = 0;
            Steps(world, 1);
            Expect.Equal(13, fake.Reads, "13 units were read in the first step");
            Expect.Equal(13, Moved(units), "and 13 entries are at the new place");
            Steps(world, 1);
            Expect.Equal(26, Moved(units), "26 after two steps");
            Steps(world, 1);
            Expect.Equal(39, Moved(units), "39 after three");
            Steps(world, 1);
            Expect.Equal(40, Moved(units), "all 40 within four steps");
            Expect.Equal(13 * 4, fake.Reads, "13 reads in each of the four steps");
            world.MoveUnit(units[5].Handle, -900.0f, 900.0f);
            Steps(world, 4);
            Expect.Equal(7 * 8, Tracked(units[5]).Cell, "a unit that moved across the map is in the cell of its place within 4 steps");
        }

        /// <summary>How many of the units have their entry at x = 100.</summary>
        private static int Moved(List<Unit> units)
        {
            var count = 0;
            for (var i = 0; i < units.Count; i++)
            {
                if (Tracked(units[i]).X == 100.0f)
                {
                    count++;
                }
            }

            return count;
        }

        /// <summary>Few units are read a unit a step at least, never more than there are, and a count that is a whole number of reads does not round up.</summary>
        public static void CellCheckCounts(FakeWorld world)
        {
            var fake = Fresh();
            UnitAt(world, 0.0f, 0.0f);
            Tracking.Need(Tracking.UnitKind);
            fake.Reads = 0;
            Steps(world, 1);
            Expect.Equal(1, fake.Reads, "one unit is read in a step");
            for (var i = 0; i < 31; i++)
            {
                UnitAt(world, 0.0f, 0.0f);
            }

            fake.Reads = 0;
            Steps(world, 1);
            Expect.Equal(10, fake.Reads, "32 units: 10 a step, as 32 * 0.03125 / 0.1 is 10");
        }

        /// <summary>A unit that ends in the middle of the list does not make the cell checks skip a unit: every unit left is read within the next 4 steps.</summary>
        public static void CellCheckAfterEnding(FakeWorld world)
        {
            Fresh();
            var units = new List<Unit>();
            for (var i = 0; i < 40; i++)
            {
                units.Add(UnitAt(world, 0.0f, 0.0f));
            }

            Tracking.Need(Tracking.UnitKind);
            Steps(world, 1);
            units[3].Remove();
            units[20].Remove();
            units.RemoveAt(20);
            units.RemoveAt(3);
            for (var i = 0; i < units.Count; i++)
            {
                world.MoveUnit(units[i].Handle, 100.0f, 0.0f);
            }

            Steps(world, 4);
            Expect.Equal(38, Moved(units), "every unit left was read");
            Expect.Equal(38, Tracking.Count, "and 38 are tracked");
        }

        /// <summary>A unit that is dead, hidden or loaded, an item that is carried or not visible, and a destructable that is dead have no place; a unit that is alive and shown, an item on the ground and a destructable that is alive have one.</summary>
        public static void NoPlace(FakeWorld world)
        {
            var fake = Fresh();
            var alive = UnitAt(world, 0.0f, 0.0f);
            var dead = UnitAt(world, 0.0f, 0.0f);
            var hidden = UnitAt(world, 0.0f, 0.0f);
            var loaded = UnitAt(world, 0.0f, 0.0f);
            var carried = ItemAt(world, 0.0f, 0.0f);
            var unseen = ItemAt(world, 0.0f, 0.0f);
            var lying = ItemAt(world, 0.0f, 0.0f);
            world.PlacedItems.Add(carried.Handle);
            world.PlacedItems.Add(unseen.Handle);
            world.PlacedItems.Add(lying.Handle);
            var tree = world.NewDestructable(ObjectTypes.FourCC("dfoo"));
            var stump = world.NewDestructable(ObjectTypes.FourCC("dfoo"));
            world.PlacedDestructables.Add(tree);
            world.PlacedDestructables.Add(stump);
            world.SlayUnit(dead.Handle);
            fake.Hidden.Add(hidden.Handle);
            fake.Loaded.Add(loaded.Handle);
            alive.Give(carried);
            fake.Unseen.Add(unseen.Handle);
            fake.Dead.Add(stump);
            Tracking.NeedFor(typeof(Actor));
            Expect.True(Tracked(alive).HasPlace, "a unit that is alive has a place");
            Expect.True(!Tracked(dead).HasPlace, "a dead unit has none");
            Expect.True(!Tracked(hidden).HasPlace, "a hidden unit has none");
            Expect.True(!Tracked(loaded).HasPlace, "a loaded unit has none");
            Expect.True(!Tracked(carried).HasPlace, "a carried item has none");
            Expect.True(!Tracked(unseen).HasPlace, "an item that is not visible has none");
            Expect.True(Tracked(lying).HasPlace, "an item on the ground has one");
            Expect.True(Tracked(Destructable.Of(tree)!).HasPlace, "a destructable that is alive has one");
            Expect.True(!Tracked(Destructable.Of(stump)!).HasPlace, "a dead destructable has none");
        }

        /// <summary>A unit that was hidden and is shown again has a place again, and the place it is at, at its next cell check.</summary>
        public static void Revived(FakeWorld world)
        {
            var fake = Fresh();
            var unit = UnitAt(world, 0.0f, 0.0f);
            fake.Hidden.Add(unit.Handle);
            Tracking.Need(Tracking.UnitKind);
            var entry = Tracked(unit);
            Expect.True(!entry.HasPlace, "hidden: no place");
            world.MoveUnit(unit.Handle, 200.0f, 50.0f);
            Steps(world, 4);
            Expect.True(!entry.HasPlace, "still hidden after four steps");
            Expect.Near(0.0f, entry.X, "and its place is not read");
            fake.Hidden.Remove(unit.Handle);
            Steps(world, 4);
            Expect.True(entry.HasPlace, "shown: a place again");
            Expect.Near(200.0f, entry.X, "X is read");
            Expect.Near(50.0f, entry.Y, "Y is read");
            Expect.Equal(4 * 8 + 4, entry.Cell, "and the cell follows");
        }

        /// <summary>Items that lie on the world are tracked when items are asked for, those the runtime knows and those it does not; the place of an item is read again at the cell checks.</summary>
        public static void ItemsFilled(FakeWorld world)
        {
            var known = ItemAt(world, 10.0f, 10.0f);
            var unknown = world.NewItem(ObjectTypes.FourCC("ofoo"));
            world.MoveItem(unknown, 300.0f, 10.0f);
            world.PlacedItems.Add(known.Handle);
            world.PlacedItems.Add(unknown);
            Fresh();
            Tracking.Need(Tracking.ItemKind);
            var first = Tracked(known);
            Expect.Equal(36, first.Cell, "the known item is in its cell");
            Expect.Equal(Tracking.ItemKind, first.Kind, "of the item kind");
            var taken = KnownItems.Find(unknown);
            Expect.True(taken != null, "the item the runtime did not know is taken in");
            Expect.Equal(37, Tracked(taken!).Cell, "and is in its cell");
            world.MoveItem(known.Handle, -500.0f, -500.0f);
            Steps(world, 4);
            Expect.Equal(2 * 8 + 2, first.Cell, "an item that moved is in the cell of its place within 4 steps");
            var made = Item.Create(ObjectTypes.FourCC("ofoo"), 700.0f, 700.0f, Game.Instance);
            Expect.True(Tracking.EntryOf(made) != null, "an item made later comes in through the hook");
        }

        /// <summary>The destructables of the world are tracked when destructables are asked for, with the place they were taken in at; they are not read again at the cell checks, and one that dies has no place at the next read.</summary>
        public static void DestructablesFilled(FakeWorld world)
        {
            var fake = Fresh();
            var tree = world.NewDestructable(ObjectTypes.FourCC("dfoo"));
            world.PlaceDestructable(tree, 300.0f, 10.0f);
            world.PlacedDestructables.Add(tree);
            Tracking.Need(Tracking.DestructableKind);
            var found = Destructable.Of(tree)!;
            var entry = Tracked(found);
            Expect.Equal(37, entry.Cell, "the cell of the destructable");
            Expect.Near(300.0f, entry.X, "its X");
            Expect.Near(10.0f, entry.Y, "its Y");
            Expect.True(entry.HasPlace, "it has a place");
            Expect.Equal(Tracking.DestructableKind, entry.Kind, "of the destructable kind");
            fake.Reads = 0;
            Steps(world, 8);
            Expect.Equal(0, fake.Reads, "the cell checks do not read it");
            fake.Dead.Add(tree);
            Tracking.ReadNow(entry);
            Expect.True(!entry.HasPlace, "a read finds that it died");
            Expect.Near(300.0f, entry.X, "and its place is where it was");
            found.Remove();
            Expect.Equal(-1, entry.Cell, "a removed destructable leaves the grid");
            var later = Destructable.Of(world.NewDestructable(ObjectTypes.FourCC("dfoo")))!;
            Expect.True(Tracking.EntryOf(later) != null, "one made later is tracked");
        }

        /// <summary>A unit, item or destructable that ends leaves the grid at once, before its OnLeave runs, and is not read again; with items asked for, the cycle runs on when nothing is tracked.</summary>
        public static void Ending(FakeWorld world)
        {
            Fresh();
            ObjectTypes.Unit(ObjectTypes.FourCC("h006"), () => new LeaveWatch());
            var unitHandle = world.NewUnit(ObjectTypes.FourCC("h006"));
            world.MoveUnit(unitHandle, 10.0f, 10.0f);
            var unit = (LeaveWatch)Unit.Of(unitHandle)!;
            var item = ItemAt(world, 10.0f, 10.0f);
            var handle = world.NewDestructable(ObjectTypes.FourCC("dfoo"));
            world.PlacedItems.Add(item.Handle);
            world.PlacedDestructables.Add(handle);
            var destructable = Destructable.Of(handle)!;
            Tracking.NeedFor(typeof(Actor));
            var unitEntry = Tracked(unit);
            var itemEntry = Tracked(item);
            var destructableEntry = Tracked(destructable);
            Expect.Equal(3, Tracking.Count, "three tracked");
            var cellInLeave = 0;
            unit.Leaving = () => { cellInLeave = unitEntry.Cell; };
            unit.Remove();
            Expect.Equal(-1, cellInLeave, "a removed unit is out of the grid when its OnLeave runs");
            Expect.Equal(-1, unitEntry.Cell, "and after");
            Expect.True(!unitEntry.HasPlace, "and has no place");
            Expect.True(Tracking.EntryOf(unit) == null, "and no entry");
            Expect.Equal(2, Tracking.Count, "two tracked");
            item.Remove();
            Expect.Equal(-1, itemEntry.Cell, "a removed item is out of the grid");
            destructable.Remove();
            Expect.Equal(-1, destructableEntry.Cell, "a removed destructable is out of the grid");
            Expect.Equal(0, Tracking.Count, "nothing tracked");
            Expect.True(Tracking.Running, "the cycle still looks for items, as items were asked for");
            Steps(world, 8);
            Expect.True(!unitEntry.HasPlace, "an entry that has ended is not read again");
        }

        /// <summary>A unit the package never saw, taken in before it was installed or never asked for, ends without a word, and with units asked for, the last unit ending stops the timer.</summary>
        public static void EndingUnseen(FakeWorld world)
        {
            var before = UnitAt(world, 0.0f, 0.0f);
            Fresh();
            var after = UnitAt(world, 0.0f, 0.0f);
            before.Remove();
            after.Remove();
            Expect.Equal(0, world.Logged.Count, "no error was logged");
            Expect.True(!Tracking.Running, "nothing runs");
            var asked = UnitAt(world, 0.0f, 0.0f);
            Tracking.Need(Tracking.UnitKind);
            Expect.True(Tracking.Running, "the cycle runs with a unit tracked");
            asked.Remove();
            Expect.True(!Tracking.Running, "and stops with the last one");
            Expect.Equal(0, Tracking.Count, "nothing tracked");
        }

        /// <summary>An item the game made with no Moonwell code is taken in and tracked within a second, while items are asked for; the cycle looks for them even when no item is tracked.</summary>
        public static void NewGroundItem(FakeWorld world)
        {
            Fresh();
            Tracking.Need(Tracking.ItemKind);
            Expect.Equal(0, Tracking.Count, "no item to track");
            Expect.True(Tracking.Running, "the cycle runs to look for one");
            var handle = world.NewItem(ObjectTypes.FourCC("ofoo"));
            world.MoveItem(handle, 50.0f, 60.0f);
            world.PlacedItems.Add(handle);
            Steps(world, 31);
            Expect.True(KnownItems.Find(handle) == null, "not found before the second is out");
            Steps(world, 1);
            var found = KnownItems.Find(handle);
            Expect.True(found != null, "found within a second");
            var entry = Tracked(found!);
            Expect.Near(50.0f, entry.X, "its X");
            Expect.Near(60.0f, entry.Y, "its Y");
            Expect.True(entry.HasPlace, "and it has a place");
            var other = world.NewItem(ObjectTypes.FourCC("ofoo"));
            world.PlacedItems.Add(other);
            Steps(world, 32);
            Expect.Equal(2, Tracking.Count, "the next second finds the next item");
            Expect.Same(entry, Tracking.EntryOf(found!), "and the first is not tracked twice");
        }

        /// <summary>Inside a step a place is read at most once, however many ask: the second ask finds what the first read, even if the thing has moved.</summary>
        public static void ReadOnce(FakeWorld world)
        {
            var fake = Fresh();
            var unit = UnitAt(world, 10.0f, 10.0f);
            Tracking.Need(Tracking.UnitKind);
            var entry = Tracked(unit);
            var well = new Well(Interactions.Step);
            var readsAfterFirst = -1;
            var readsAfterSecond = -1;
            well.Ticking = () =>
            {
                fake.Reads = 0;
                world.MoveUnit(unit.Handle, 500.0f, 0.0f);
                Tracking.ReadNow(entry);
                readsAfterFirst = fake.Reads;
                world.MoveUnit(unit.Handle, 600.0f, 0.0f);
                Tracking.ReadNow(entry);
                readsAfterSecond = fake.Reads;
                well.Ticking = null;
            };
            Gizmo.Create(well, 0.0f, 0.0f, Game.Instance);
            Steps(world, 1);
            Expect.Equal(1, readsAfterFirst, "the first ask read the unit");
            Expect.Equal(1, readsAfterSecond, "the second read nothing");
            Expect.Near(500.0f, entry.X, "it holds what the first read, also after the cell check of the same step");
            Steps(world, 1);
            Expect.Near(600.0f, entry.X, "the next step reads it again");
        }

        /// <summary>A tracked thing told it was moved takes the place for this step and its cell at once: a Find in the same step sees it there, though it was read earlier in the step. A thing not tracked is passed over.</summary>
        public static void Moved(FakeWorld world)
        {
            Fresh();
            var unit = UnitAt(world, 10.0f, 10.0f);
            Tracking.Need(Tracking.UnitKind);
            var entry = Tracked(unit);
            var well = new Well(Interactions.Step);
            var near = -1;
            var far = -1;
            var step = -1;
            var stale = -1;
            well.Ticking = () =>
            {
                Tracking.ReadNow(entry);
                world.MoveUnit(unit.Handle, 1000.0f, -1000.0f);
                stale = Find.InRange<Unit>(1000.0f, -1000.0f, 10.0f).Count;
                Tracking.Moved(unit, 1000.0f, -1000.0f);
                near = Find.InRange<Unit>(1000.0f, -1000.0f, 10.0f).Count;
                far = Find.InRange<Unit>(10.0f, 10.0f, 10.0f).Count;
                step = entry.ReadStep;
                well.Ticking = null;
            };
            Gizmo.Create(well, 0.0f, 0.0f, Game.Instance);
            Steps(world, 1);
            Expect.Equal(0, stale, "a unit read earlier in the step is still where it was read");
            Expect.Equal(1, near, "after Moved a Find in the same step finds it at its new place");
            Expect.Equal(0, far, "and not at the old one");
            Expect.Equal(Tracking.StepNow, step, "its entry was read for this step");
            Expect.Near(1000.0f, entry.X, "X");
            Expect.Near(-1000.0f, entry.Y, "Y");
            Expect.Equal(7, entry.Cell, "and the cell is the new place's");
            Steps(world, 1);
            Expect.Near(1000.0f, entry.X, "the next step reads the unit where the game has it");
            var plain = Gizmo.Create(new Well(), 0.0f, 0.0f, Game.Instance);
            Tracking.Moved(plain, 50.0f, 50.0f);
            Expect.Near(50.0f, plain.X, "a gizmo has its place set");
            var gone = UnitAt(world, 30.0f, 30.0f);
            Tracking.Reset();
            Tracking.Moved(gone, 1.0f, 1.0f);
            Expect.Equal(0, Tracking.Count, "a thing not tracked is passed over, and nothing is tracked by it");
            Expect.True(Tracking.EntryOf(gone) == null, "and it is still not tracked");
        }

        /// <summary>Outside a running step there is no step to read once in: every ask reads the place, so a Find from a button sees a unit that has just been moved.</summary>
        public static void ReadOutsideStep(FakeWorld world)
        {
            var fake = Fresh();
            var unit = UnitAt(world, 10.0f, 10.0f);
            Tracking.Need(Tracking.UnitKind);
            var entry = Tracked(unit);
            world.MoveUnit(unit.Handle, 200.0f, 0.0f);
            fake.Reads = 0;
            Tracking.ReadNow(entry);
            Expect.Near(200.0f, entry.X, "the first ask reads");
            world.MoveUnit(unit.Handle, 300.0f, 0.0f);
            Tracking.ReadNow(entry);
            Expect.Near(300.0f, entry.X, "and so does the second, in the same moment");
            Expect.Equal(2, fake.Reads, "two reads");
            Expect.Equal(4 * 8 + 5, entry.Cell, "the cell follows the read");
        }

        /// <summary>A gizmo always has its place, and a read changes nothing of it.</summary>
        public static void GizmoPlace(FakeWorld world)
        {
            var fake = Fresh();
            var well = Gizmo.Create(new Well(), 10.0f, 10.0f, Game.Instance);
            var entry = Tracked(well);
            Tracking.ReadNow(entry);
            Expect.True(entry.HasPlace, "a gizmo has a place");
            Expect.Near(10.0f, entry.X, "which is its own");
            Expect.Equal(0, fake.Reads, "and nothing is read of the game");
            Expect.Equal(Tracking.GizmoKind, entry.Kind, "of the gizmo kind");
        }

        /// <summary>The first use asking for items, with nothing installed before it, makes one cycle timer: a gizmo that ticks every step ticks once a step, and items stay asked for.</summary>
        public static void FirstUseItems(FakeWorld world)
        {
            var item = ItemAt(world, 10.0f, 10.0f);
            world.PlacedItems.Add(item.Handle);
            FirstUse(new FakeInteractionWorld(), () =>
            {
                Expect.Equal(1, Find.InRange<Item>(0.0f, 0.0f, 100.0f).Count, "the first Find finds the item");
                var well = Gizmo.Create(new Well(Interactions.Step), 0.0f, 0.0f, Game.Instance);
                Steps(world, 4);
                Expect.Equal(4, well.TickSteps.Count, "four ticks in four steps");
                Expect.True(Tracking.Asked(Tracking.ItemKind), "items are asked for");
            });
        }

        /// <summary>The first use asking for units, with nothing installed before it, tracks the units there are and keeps units asked for: a unit made after comes in through the hook.</summary>
        public static void FirstUseUnits(FakeWorld world)
        {
            UnitAt(world, 10.0f, 10.0f);
            UnitAt(world, 20.0f, 10.0f);
            FirstUse(new FakeInteractionWorld(), () =>
            {
                Expect.Equal(2, Find.InRange<Unit>(0.0f, 0.0f, 100.0f).Count, "the first Find finds both units");
                var later = UnitAt(world, 30.0f, 10.0f);
                Expect.True(Tracking.EntryOf(later) != null, "a unit made after is tracked");
                Expect.True(Tracking.Asked(Tracking.UnitKind), "units are asked for");
                Expect.Equal(3, Tracking.Count, "three tracked");
            });
        }

        /// <summary>An install over one whose cycle runs ends that cycle's timer: a gizmo made after ticks once a step.</summary>
        public static void InstallAgain(FakeWorld world)
        {
            Fresh();
            Gizmo.Create(new Well(Interactions.Step), 0.0f, 0.0f, Game.Instance);
            Expect.True(Tracking.Running, "the first install's cycle runs");
            Fresh();
            var well = Gizmo.Create(new Well(Interactions.Step), 0.0f, 0.0f, Game.Instance);
            Steps(world, 4);
            Expect.Equal(4, well.TickSteps.Count, "four ticks in four steps");
        }

        /// <summary>
        /// A read of a unit, which the package writes out for speed (in Lua it skips the getters and casts and works the cell
        /// with math.floor): a dead, hidden or loaded unit has no place and keeps its last one; a move inside a cell keeps the
        /// cell, and one onto a cell's edge takes the unit to the end of that cell's list, with the edge cell for a place past
        /// the world; a unit that reads x and y 0 with type id 0 keeps its last place, and X gives it
        /// after, as Unit.X does; inside Local.For the read keeps no last place on the unit.
        /// </summary>
        public static void UnitRead(FakeWorld world)
        {
            var fake = Fresh();
            var unit = UnitAt(world, 10.0f, 10.0f);
            var other = UnitAt(world, 300.0f, 10.0f);
            Tracking.Need(Tracking.UnitKind);
            var entry = Tracked(unit);
            var otherEntry = Tracked(other);
            var grid = Interactions.Grid;
            world.MoveUnit(unit.Handle, 20.0f, 30.0f);
            Tracking.ReadNow(entry);
            Expect.Equal(36, entry.Cell, "a move inside its cell keeps the cell");
            Expect.Near(20.0f, entry.X, "and reads the new x");
            Expect.Near(30.0f, entry.Y, "and y");
            world.MoveUnit(unit.Handle, 256.0f, 10.0f);
            Tracking.ReadNow(entry);
            Expect.Equal(37, entry.Cell, "a place on a cell's left edge is in that cell");
            Expect.Same(otherEntry, grid.CellList(37)![0], "the unit there first stays first");
            Expect.Same(entry, grid.CellList(37)![1], "the one moved in is last");
            Expect.Equal(0, grid.CellList(36)!.Count, "and the old cell is empty");
            world.MoveUnit(unit.Handle, 5000.0f, -5000.0f);
            Tracking.ReadNow(entry);
            Expect.Equal(7, entry.Cell, "a place past the world is in the edge cell");
            Expect.Near(5000.0f, entry.X, "and keeps its own x");
            Expect.Near(5000.0f, unit.X, "the unit's x is the one read");
            world.MoveUnit(unit.Handle, 0.0f, 0.0f);
            world.VanishUnit(unit.Handle);
            Tracking.ReadNow(entry);
            Expect.True(entry.HasPlace, "a unit that reads type id 0 but is alive still has a place");
            Expect.Near(5000.0f, entry.X, "and its x 0 is taken for the last x read");
            Expect.Near(-5000.0f, entry.Y, "and its y 0 for the last y");
            Expect.Equal(7, entry.Cell, "so it stays in its cell");
            Expect.Near(5000.0f, unit.X, "and Unit.X gives the last x");
            world.MoveUnit(other.Handle, 100.0f, 900.0f);

            // Not a path the package takes (inside Local.For a search peeks and calls no ReadNow): it checks that the read
            // keeps Local.For's rule for the last place, as Unit.X does.
            Local.For(Player.Get(world.LocalSlot), () => Tracking.ReadNow(otherEntry));
            Expect.Near(100.0f, otherEntry.X, "a read inside Local.For reads the place");
            world.MoveUnit(other.Handle, 0.0f, 0.0f);
            world.VanishUnit(other.Handle);
            Expect.Near(300.0f, other.X, "but keeps no last x on the unit");
            world.MoveUnit(other.Handle, 0.0f, 0.0f);
            fake.Hidden.Add(other.Handle);
            Tracking.ReadNow(otherEntry);
            Expect.True(!otherEntry.HasPlace, "a hidden unit has no place");
            Expect.Near(100.0f, otherEntry.X, "and keeps the place it was last read at");
            fake.Hidden.Clear();
            fake.Loaded.Add(other.Handle);
            Tracking.ReadNow(otherEntry);
            Expect.True(!otherEntry.HasPlace, "a loaded unit has none");
            fake.Loaded.Clear();
            Tracking.ReadNow(otherEntry);
            Expect.True(otherEntry.HasPlace, "shown and unloaded, it has a place again");
            var victim = UnitAt(world, -300.0f, 10.0f);
            var victimEntry = Tracked(victim);
            world.SlayUnit(victim.Handle);
            Tracking.ReadNow(victimEntry);
            Expect.True(!victimEntry.HasPlace, "a dead unit has none");
        }

        /// <summary>A cell check whose read fails, here the world's, is reported as Interactions.CellCheck and goes no further that step, the interactions after it in the step still check, and the round goes on from the unit after at the next step.</summary>
        public static void CellCheckFails(FakeWorld world)
        {
            var fake = Fresh();
            var failing = UnitAt(world, -900.0f, -900.0f);
            var other = UnitAt(world, 200.0f, 0.0f);
            Tracking.Need(Tracking.UnitKind);
            fake.FailingUnits.Add(failing.Handle);
            var well = Gizmo.Create(new Well(), 100.0f, 0.0f, Game.Instance);
            var healing = well.Interact(new Healing(), Game.Instance);
            healing.Tag = unit => unit == other ? "other" : "?";
            TestLog.Lines.Clear();
            Steps(world, 1);
            Expect.True(Logged(world, "Interactions.CellCheck failed"), "the failed read is reported as Interactions.CellCheck");
            Expect.True(!Logged(world, ".Timer failed"), "and nothing went on up to the cycle's timer");
            Expect.True(TestLog.Lines.Count > 0 && TestLog.Lines[0] == "meet other", "the interaction after it checked in the same step");
            healing.End();
            world.MoveUnit(other.Handle, 250.0f, 0.0f);
            Steps(world, 1);
            Expect.Near(250.0f, Tracked(other).X, "with nothing else reading it, the next step's cell check read the unit after the one that failed");
        }

        /// <summary>Whether a line of the game's log has the text.</summary>
        private static bool Logged(FakeWorld world, string text)
        {
            foreach (var line in world.Logged)
            {
                if (line.Contains(text))
                {
                    return true;
                }
            }

            return false;
        }
    }
}
