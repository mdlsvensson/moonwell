using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell.Testing
{
    /// <summary>Cases that check items: their classes, their components, pick-up and drop, removal and the sweep.</summary>
    public static class ItemCases
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

        /// <summary>A registered item is an object of its class, with its type id, and Of gives the same object again.</summary>
        public static void RegisteredClass(FakeWorld world)
        {
            ObjectTypes.Item(ObjectTypes.FourCC("I001"), () => new TestOrb());
            var handle = world.NewItem(ObjectTypes.FourCC("I001"));
            var found = Item.Of(handle);
            Expect.True(found is TestOrb, "the item is a TestOrb");
            Expect.Equal(ObjectTypes.FourCC("I001"), found!.TypeId, "the type id");
            Expect.Same(found, Item.Of(handle), "the item asked for again");
        }

        /// <summary>An unregistered item is a plain Item.</summary>
        public static void FallbackClass(FakeWorld world)
        {
            Expect.Equal("Item", Names.NameOf(Item.Of(world.NewItem(ObjectTypes.FourCC("ofoo")))!), "an unregistered item");
        }

        /// <summary>Picking up an item runs the item's OnPickedUp, its components, then the unit's OnItemPickedUp and its components.</summary>
        public static void PickupOrder(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("H001"), () => new TestHero());
            ObjectTypes.Item(ObjectTypes.FourCC("I001"), () => new TestOrb());
            var hero = Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), hero: true))!;
            var orb = world.NewItem(ObjectTypes.FourCC("I001"));
            TestLog.Lines.Clear();
            Inbound.PickedUp(hero.Handle, orb);
            ExpectLog(new string[] { "orb picked", "glow picked", "hero got orb", "tally got orb" }, "the log");
        }

        /// <summary>Dropping an item runs the item's components' OnDropped, then the unit's OnItemDropped and its components.</summary>
        public static void DropOrder(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("H001"), () => new TestHero());
            ObjectTypes.Item(ObjectTypes.FourCC("I001"), () => new TestOrb());
            var hero = Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), hero: true))!;
            var orb = world.NewItem(ObjectTypes.FourCC("I001"));
            TestLog.Lines.Clear();
            Inbound.Dropped(hero.Handle, orb);
            ExpectLog(new string[] { "glow dropped", "hero dropped orb", "tally dropped orb" }, "the log");
        }

        /// <summary>The runtime listens for pick-ups and drops from its start, before any item is taken in, since keeping an item's carrier needs them and listening must not begin in code that only one machine runs.</summary>
        public static void Listens(FakeWorld world)
        {
            Expect.True(world.Listening.Contains(GameEvent.PickUp), "PickUp is listened for from the start");
            Expect.True(world.Listening.Contains(GameEvent.Drop), "Drop is listened for from the start");
        }

        /// <summary>An item that vanishes is ended by the sweep, and the game is not asked to remove it.</summary>
        public static void SweepDetectsRemoval(FakeWorld world)
        {
            ObjectTypes.Item(ObjectTypes.FourCC("I001"), () => new TestOrb());
            var handle = world.NewItem(ObjectTypes.FourCC("I001"));
            var orb = Item.Of(handle)!;
            world.VanishItem(handle);
            world.Advance(0.3f);
            Expect.True(orb.HasEnded, "the vanished item has ended");
            Expect.True(orb.IsGone, "the vanished item is gone");
            Expect.Equal(0, world.RemovedItems.Count, "the items the game was asked to remove");
        }

        /// <summary>An item created for an owner has that owner, and is removed from the game when the owner ends.</summary>
        public static void OwnedByOwner(FakeWorld world)
        {
            var log = new List<string>();
            var owner = new TestOwner("owner", log);
            var made = Item.Create(ObjectTypes.FourCC("ofoo"), 0f, 0f, owner);
            Expect.Same(owner, made.Owner, "the owner of the item");
            owner.End();
            Expect.Equal(1, world.RemovedItems.Count, "the items the game was asked to remove");
            Expect.Same(made.Handle, world.RemovedItems[0], "the handle removed");
        }

        /// <summary>A hero's inventory: Give puts an item in the first free slot, ItemIn reads a slot (0-based), Drop takes the item out, and the pick-up and the drop come back through the game's events, once each.</summary>
        public static void Inventory(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("H001"), () => new TestHero());
            ObjectTypes.Item(ObjectTypes.FourCC("I001"), () => new TestOrb());
            var hero = Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), hero: true))!;
            var orb = Item.Of(world.NewItem(ObjectTypes.FourCC("I001")))!;
            Expect.Equal(6, hero.InventorySize, "the inventory size");
            Expect.True(hero.ItemIn(0) == null, "slot 0 before");
            TestLog.Lines.Clear();

            Expect.True(hero.Give(orb), "the game accepts the item");

            ExpectLog(new string[] { "orb picked", "glow picked", "hero got orb", "tally got orb" }, "the log after Give");
            Expect.Same(orb, hero.ItemIn(0), "slot 0 after Give");
            Expect.True(hero.ItemIn(1) == null, "slot 1 after Give");
            TestLog.Lines.Clear();

            hero.Drop(orb);

            ExpectLog(new string[] { "glow dropped", "hero dropped orb", "tally dropped orb" }, "the log after Drop");
            Expect.True(hero.ItemIn(0) == null, "slot 0 after Drop");
        }

        /// <summary>Give gives the game's answer when the inventory is full, and ItemIn gives null and does not ask the game for a slot the inventory does not have.</summary>
        public static void InventoryLimits(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            world.SetInventorySize(handle, 1);
            var unit = Unit.Of(handle)!;
            var first = Item.Of(world.NewItem(ObjectTypes.FourCC("ofoo")))!;
            var second = Item.Of(world.NewItem(ObjectTypes.FourCC("ofoo")))!;

            Expect.Equal(1, unit.InventorySize, "the inventory size");
            Expect.True(unit.Give(first), "the first item fits");
            Expect.True(!unit.Give(second), "the second item does not");
            Expect.Same(first, unit.ItemIn(0), "slot 0");

            world.ItemSlotReads = 0;
            Expect.True(unit.ItemIn(1) == null, "slot 1 is out of range");
            Expect.True(unit.ItemIn(-1) == null, "slot -1 is out of range");
            Expect.True(unit.ItemIn(6) == null, "slot 6 is out of range");
            Expect.Equal(0, world.ItemSlotReads, "slots read from the game");
        }

        /// <summary>A removed unit has the last inventory size read, no items, takes no item and drops none; a removed item is not given. The game is not called.</summary>
        public static void InventoryGone(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var unit = Unit.Of(handle)!;
            var carried = Item.Of(world.NewItem(ObjectTypes.FourCC("ofoo")))!;
            var spare = Item.Of(world.NewItem(ObjectTypes.FourCC("ofoo")))!;
            var lost = world.NewItem(ObjectTypes.FourCC("ofoo"));
            var lostItem = Item.Of(lost)!;
            var holder = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            Expect.Equal(6, unit.InventorySize, "the inventory size read");
            Expect.True(unit.Give(carried), "the item fits");
            world.VanishItem(lost);
            world.Advance(0.3f);
            Expect.True(lostItem.IsGone, "the sweep found the item gone");
            world.InventoryCalls.Clear();

            Expect.True(!holder.Give(lostItem), "a removed item is not given");
            holder.Drop(lostItem);
            Expect.Equal(0, world.InventoryCalls.Count, "no inventory call for a removed item");

            unit.Remove();
            world.GoneCalls = 0;
            world.ItemSlotReads = 0;

            Expect.Equal(6, unit.InventorySize, "the last inventory size");
            Expect.True(unit.ItemIn(0) == null, "no item in a removed unit");
            Expect.True(!unit.Give(spare), "a removed unit takes no item");
            unit.Drop(carried);
            Expect.Equal(0, world.InventoryCalls.Count, "no inventory call for the removed unit");
            Expect.Equal(0, world.ItemSlotReads, "no slot read for the removed unit");
            Expect.Equal(0, world.GoneCalls, "no call to the game for the gone unit");
        }

        /// <summary>An item's charges are the game's: read and set, and a set keeps what was given.</summary>
        public static void Charges(FakeWorld world)
        {
            var handle = world.NewItem(ObjectTypes.FourCC("ofoo"));
            var item = Item.Of(handle)!;
            Expect.Equal(1, item.Charges, "the charges before");

            item.Charges = 5;

            Expect.Equal(5, item.Charges, "the new charges");
            Expect.Equal(5, world.ChargesOf(handle), "the game's charges");
            Expect.Equal(1, world.ItemCalls.Count, "calls that changed the item");
            Expect.Equal("charges 5", world.ItemCalls[0], "the call");

            item.Charges = 0;
            Expect.Equal(0, item.Charges, "the charges used up");
        }

        /// <summary>An item's player is the game's: read and set, and the item's colour is left alone.</summary>
        public static void PlayerOf(FakeWorld world)
        {
            var handle = world.NewItem(ObjectTypes.FourCC("ofoo"));
            var item = Item.Of(handle)!;
            Expect.Equal(0, item.Player.Slot, "the player before");

            item.Player = Player.Get(3);

            Expect.Equal(3, item.Player.Slot, "the new player");
            Expect.Equal(3, world.SlotOf(world.ItemOwnerOf(handle)), "the game's player");
            Expect.Equal("owner 3 False", world.ItemCalls[0], "the call keeps the colour");
        }

        /// <summary>An item is created at its place; X and Y read it, and MoveTo moves it.</summary>
        public static void Position(FakeWorld world)
        {
            var item = Item.Create(ObjectTypes.FourCC("ofoo"), 100.5f, -50.0f);
            Expect.Near(100.5f, item.X, "x at creation");
            Expect.Near(-50.0f, item.Y, "y at creation");

            item.MoveTo(-300.0f, 64.25f);

            Expect.Near(-300.0f, item.X, "the new x");
            Expect.Near(64.25f, item.Y, "the new y");
            Expect.Equal(1, world.ItemCalls.Count, "calls that changed the item");
            Expect.Equal("move", world.ItemCalls[0], "the call");
        }

        /// <summary>The carrier is the unit that picked the item up, until a drop; an item nobody has picked up has none.</summary>
        public static void Carrier(FakeWorld world)
        {
            var hero = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var orb = Item.Of(world.NewItem(ObjectTypes.FourCC("ofoo")))!;
            Expect.True(!orb.IsCarried, "carried before");
            Expect.True(orb.Carrier == null, "no carrier before");

            hero.Give(orb);

            Expect.True(orb.IsCarried, "carried after Give");
            Expect.Same(hero, orb.Carrier, "the carrier after Give");

            hero.Drop(orb);

            Expect.True(!orb.IsCarried, "carried after Drop");
            Expect.True(orb.Carrier == null, "no carrier after Drop");
        }

        /// <summary>The carrier is already set when the item's own OnPickedUp runs; inside OnDropped it is what the game says then, and the item is already out of the inventory, so null.</summary>
        public static void CarrierInHooks(FakeWorld world)
        {
            ObjectTypes.Item(ObjectTypes.FourCC("I001"), () => new CarrierWatch());
            var hero = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var watch = (CarrierWatch)Item.Of(world.NewItem(ObjectTypes.FourCC("I001")))!;

            hero.Give(watch);
            Expect.Same(hero, watch.CarrierAtPickup, "the carrier seen in OnPickedUp");

            hero.Drop(watch);
            Expect.True(watch.DropSeen, "OnDropped ran");
            Expect.True(watch.CarrierAtDrop == null, "the carrier seen in OnDropped");
        }

        /// <summary>A carrier that dies drops the item (the game tells Moonwell as for any drop), so the item has no carrier then.</summary>
        public static void CarrierDies(FakeWorld world)
        {
            var hero = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var orb = Item.Of(world.NewItem(ObjectTypes.FourCC("ofoo")))!;
            hero.Give(orb);
            Expect.Same(hero, orb.Carrier, "the carrier before the death");

            hero.Kill();

            Expect.True(orb.Carrier == null, "no carrier after the carrier died");
            Expect.True(!orb.IsCarried, "not carried after the carrier died");
        }

        /// <summary>An item Moonwell never saw picked up has no carrier, even when ItemIn finds it in a slot; a carrier the game has removed carries nothing.</summary>
        public static void CarrierUnseen(FakeWorld world)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var unit = Unit.Of(handle)!;
            var placed = world.NewItem(ObjectTypes.FourCC("ofoo"));
            world.PutInSlot(handle, 0, placed);

            var found = unit.ItemIn(0)!;

            Expect.True(found.Carrier == null, "no carrier for an item never seen picked up");

            var given = Item.Of(world.NewItem(ObjectTypes.FourCC("ofoo")))!;
            unit.Give(given);
            Expect.Same(unit, given.Carrier, "the carrier after Give");

            unit.Remove();

            Expect.True(given.Carrier == null, "no carrier once the carrier is gone");
            Expect.True(!given.IsCarried, "not carried once the carrier is gone");
        }

        /// <summary>The drop the game fires when an item only moves between slots does not make the item uncarried: Carrier asks the game whether the unit still has it.</summary>
        public static void CarrierSlotMove(FakeWorld world)
        {
            ObjectTypes.Item(ObjectTypes.FourCC("I001"), () => new CarrierWatch());
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            var unit = Unit.Of(handle)!;
            var watch = (CarrierWatch)Item.Of(world.NewItem(ObjectTypes.FourCC("I001")))!;
            unit.Give(watch);
            watch.DropSeen = false;

            world.MoveItemSlot(handle, watch.Handle, 3);

            Expect.True(watch.DropSeen, "the game fired a drop for the move");
            Expect.Same(unit, watch.CarrierAtDrop, "the carrier seen in OnDropped for a move");
            Expect.Same(unit, watch.Carrier, "the carrier after the move");
            Expect.True(watch.IsCarried, "carried after the move");

            unit.Drop(watch);

            Expect.True(watch.Carrier == null, "no carrier after a real drop");
        }

        /// <summary>A hero that dies keeps its items (assumed, see unseen.md), so they stay carried; a unit that is not a hero drops them.</summary>
        public static void CarrierHeroDies(FakeWorld world)
        {
            var hero = Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), hero: true))!;
            var orb = Item.Of(world.NewItem(ObjectTypes.FourCC("ofoo")))!;
            hero.Give(orb);

            hero.Kill();

            Expect.Same(hero, orb.Carrier, "the hero still carries the item");
        }

        /// <summary>The player an item had when Moonwell took it in is what it gives if the game removes it before anyone reads it; an item the game gives no owner for belongs to the neutral passive player.</summary>
        public static void PlayerGoneBeforeRead(FakeWorld world)
        {
            var handle = world.NewItem(ObjectTypes.FourCC("ofoo"));
            world.SetItemOwner(handle, world.PlayerAt(3));
            var swept = Item.Of(handle)!;
            world.VanishItem(handle);
            world.Advance(0.3f);
            Expect.True(swept.IsGone, "the sweep found the item gone");
            Expect.Equal(3, swept.Player.Slot, "the player it had when taken in");

            var ownerless = world.NewItem(ObjectTypes.FourCC("ofoo"));
            world.LoseItemOwner(ownerless);
            var neutral = Item.Of(ownerless)!;
            Expect.Equal(world.NeutralPassiveSlot(), neutral.Player.Slot, "no owner given: the neutral passive player");
        }

        /// <summary>A removed item gives the last charges, player and place read or set, has no carrier, changes nothing and asks the game nothing.</summary>
        public static void ItemGoneReadsLast(FakeWorld world)
        {
            var handle = world.NewItem(ObjectTypes.FourCC("ofoo"));
            var swept = Item.Of(handle)!;
            swept.Charges = 4;
            swept.Player = Player.Get(2);
            swept.MoveTo(10.0f, 20.0f);
            Expect.Equal(4, swept.Charges, "read: charges");
            Expect.Near(10.0f, swept.X, "read: x");
            Expect.Near(20.0f, swept.Y, "read: y");
            Expect.Equal(2, swept.Player.Slot, "read: player");
            var carrier = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            carrier.Give(swept);
            Expect.True(swept.IsCarried, "carried before the removal");
            var calls = world.ItemCalls.Count;

            world.VanishItem(handle);
            world.Advance(0.3f);
            Expect.True(swept.IsGone, "the sweep found the item gone");
            world.GoneCalls = 0;
            Expect.Equal(4, swept.Charges, "swept: charges");
            Expect.Equal(2, swept.Player.Slot, "swept: player");
            Expect.Near(10.0f, swept.X, "swept: x");
            Expect.Near(20.0f, swept.Y, "swept: y");
            Expect.True(swept.Carrier == null, "swept: no carrier");
            Expect.True(!swept.IsCarried, "swept: not carried");
            ExpectNoCalls(world, swept, calls, "swept");

            var removed = Item.Create(ObjectTypes.FourCC("ofoo"), 5.0f, 6.0f);
            removed.Charges = 7;
            Expect.Equal(7, removed.Charges, "removed: charges read");
            Expect.Near(5.0f, removed.X, "removed: x read");
            Expect.Near(6.0f, removed.Y, "removed: y read");
            removed.Remove();
            world.GoneCalls = 0;
            calls = world.ItemCalls.Count;
            Expect.Equal(7, removed.Charges, "removed: charges");
            Expect.Near(5.0f, removed.X, "removed: x");
            Expect.Near(6.0f, removed.Y, "removed: y");
            ExpectNoCalls(world, removed, calls, "removed");

            // Values set and never read: the setters keep them, so they are what a gone item gives.
            var unread = Item.Of(world.NewItem(ObjectTypes.FourCC("ofoo")))!;
            unread.Charges = 9;
            unread.Player = Player.Get(4);
            unread.MoveTo(1.5f, 2.5f);
            unread.Remove();
            Expect.Equal(9, unread.Charges, "unread: charges");
            Expect.Equal(4, unread.Player.Slot, "unread: player");
            Expect.Near(1.5f, unread.X, "unread: x");
            Expect.Near(2.5f, unread.Y, "unread: y");
        }

        /// <summary>Sets every member of a gone item and checks that the game was neither asked nor told.</summary>
        private static void ExpectNoCalls(FakeWorld world, Item item, int calls, string what)
        {
            item.Charges = 99;
            item.Player = Player.Get(5);
            item.MoveTo(500.0f, 600.0f);
            Expect.Equal(calls, world.ItemCalls.Count, what + ": calls that changed the item");
            Expect.Equal(0, world.GoneCalls, what + ": calls to the game for the gone item");
        }


        /// <summary>The game removes an item and its place and charges read 0 before the sweep finds it (seen 10-21): the last place and charges stay.</summary>
        public static void GoneBeforeSweep(FakeWorld world)
        {
            var handle = world.NewItem(ObjectTypes.FourCC("ofoo"));
            var item = Item.Of(handle)!;
            item.Charges = 4;
            item.MoveTo(10.0f, 20.0f);
            Expect.Equal(4, item.Charges, "charges read");
            Expect.Near(10.0f, item.X, "x read");
            Expect.Near(20.0f, item.Y, "y read");

            world.VanishItem(handle);

            Expect.True(!item.IsGone, "the sweep has not run");
            Expect.Equal(4, item.Charges, "charges before the sweep");
            Expect.Near(10.0f, item.X, "x before the sweep");
            Expect.Near(20.0f, item.Y, "y before the sweep");
            world.Advance(0.3f);
            Expect.True(item.IsGone, "the sweep found the item gone");
            Expect.Equal(4, item.Charges, "charges after the sweep");
            Expect.Near(10.0f, item.X, "x after the sweep");
        }
    }
}
