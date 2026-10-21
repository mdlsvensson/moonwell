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

        /// <summary>An item of a registered class is created by its class, as a unit is.</summary>
        public static void CreateOfClass(FakeWorld world)
        {
            ObjectTypes.Item(ObjectTypes.FourCC("I002"), () => new TestOrb());
            var orb = Item.Create<TestOrb>(5.0f, 6.0f);
            Expect.True(orb.Has<OrbGlow>(), "the orb has its starting component");
            Expect.Near(5.0f, orb.X, "the orb's x");
            Expect.Near(6.0f, orb.Y, "the orb's y");
            Expect.Equal(ObjectTypes.FourCC("I002"), world.ItemTypeId(orb.Handle), "the orb's type in the game");
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

        /// <summary>MoveTo keeps the place the game gives, not the one asked (the game snaps it, seen 10-21), also as the last place once the game has removed the item.</summary>
        public static void MoveToKeepsThePlaceTheGameGives(FakeWorld world)
        {
            var handle = world.NewItem(ObjectTypes.FourCC("ofoo"));
            var item = Item.Of(handle)!;
            world.ItemLandsAt = new float[] { 3312.0f, 40.0f };

            item.MoveTo(5000.0f, 40.0f);

            // Nothing read in between: the last place is the one MoveTo stored.
            world.VanishItem(handle);
            world.Advance(0.3f);

            Expect.True(item.IsGone, "the sweep found the item gone");
            Expect.Near(3312.0f, item.X, "the last x of the removed item");
            Expect.Near(40.0f, item.Y, "the last y of the removed item");

            var other = Item.Of(world.NewItem(ObjectTypes.FourCC("ofoo")))!;
            other.MoveTo(5000.0f, 41.0f);
            Expect.Near(3312.0f, other.X, "the x the game gives");
            Expect.Near(40.0f, other.Y, "the y the game gives");
        }

        /// <summary>An item the game removed but the sweep has not found reads 0 (seen 10-21): MoveTo then does not overwrite the last place with that 0.</summary>
        public static void MoveToOnARemovedItemKeepsTheLastPlace(FakeWorld world)
        {
            var handle = world.NewItem(ObjectTypes.FourCC("ofoo"));
            var item = Item.Of(handle)!;
            item.MoveTo(120.0f, 45.0f);

            world.VanishItem(handle);
            Expect.True(!item.IsGone, "the sweep has not found the item yet");
            item.MoveTo(5000.0f, 41.0f);
            world.Advance(0.3f);

            Expect.True(item.IsGone, "the sweep found the item gone");
            Expect.Near(120.0f, item.X, "the last x");
            Expect.Near(45.0f, item.Y, "the last y");
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

        /// <summary>The carrier is already set when the item's own OnPickedUp runs, and inside OnDropped it is still the unit: the game takes the item out of the slot after the drop event (seen 10-21), and after it the item has no carrier.</summary>
        public static void CarrierInHooks(FakeWorld world)
        {
            ObjectTypes.Item(ObjectTypes.FourCC("I001"), () => new CarrierWatch());
            var hero = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var watch = (CarrierWatch)Item.Of(world.NewItem(ObjectTypes.FourCC("I001")))!;

            hero.Give(watch);
            Expect.Same(hero, watch.CarrierAtPickup, "the carrier seen in OnPickedUp");

            hero.Drop(watch);
            Expect.True(watch.DropSeen, "OnDropped ran");
            Expect.Same(hero, watch.CarrierAtDrop, "the carrier seen in OnDropped");
            Expect.True(watch.Carrier == null, "no carrier after the drop");
        }

        /// <summary>A carrier that is not a hero drops its item when it dies, with no drop event (seen 10-21), so the item has no carrier then.</summary>
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

        /// <summary>A hero that dies keeps its items (seen 10-21), so they stay carried and no drop event runs.</summary>
        public static void CarrierHeroDies(FakeWorld world)
        {
            ObjectTypes.Item(ObjectTypes.FourCC("I001"), () => new CarrierWatch());
            var hero = Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), hero: true))!;
            var orb = (CarrierWatch)Item.Of(world.NewItem(ObjectTypes.FourCC("I001")))!;
            hero.Give(orb);

            hero.Kill();

            Expect.Same(hero, orb.Carrier, "the hero still carries the item");
            Expect.Same(orb, hero.ItemIn(0), "the item is still in the slot");
            Expect.True(!orb.DropSeen, "no drop event");
        }

        /// <summary>A unit that is not a hero drops its items when it dies, at its place, and the game fires no drop event for them (seen 10-21).</summary>
        public static void CarrierNonHeroDiesSilently(FakeWorld world)
        {
            ObjectTypes.Item(ObjectTypes.FourCC("I001"), () => new CarrierWatch());
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            unit.PlaceAt(70.0f, 80.0f);
            var watch = (CarrierWatch)Item.Create(ObjectTypes.FourCC("I001"), 5.0f, 6.0f);
            unit.Give(watch);

            unit.Kill();

            Expect.True(!watch.DropSeen, "no drop event for the death");
            Expect.True(watch.Carrier == null, "no carrier after the death");
            Expect.True(unit.ItemIn(0) == null, "the slot is empty");
            Expect.Near(70.0f, watch.X, "the item lies at the unit's x");
            Expect.Near(80.0f, watch.Y, "the item lies at the unit's y");
        }

        /// <summary>An item picked up keeps the place it had (seen 10-21: GetItemX of a carried item is where it was picked up); a drop puts it at the unit's place.</summary>
        public static void PlaceWhileCarried(FakeWorld world)
        {
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            unit.PlaceAt(100.0f, 200.0f);
            var item = Item.Create(ObjectTypes.FourCC("ofoo"), 5.0f, 6.0f);

            unit.Give(item);
            unit.PlaceAt(300.0f, 400.0f);

            Expect.Near(5.0f, item.X, "x while carried");
            Expect.Near(6.0f, item.Y, "y while carried");

            unit.Drop(item);

            Expect.Near(300.0f, item.X, "x after the drop");
            Expect.Near(400.0f, item.Y, "y after the drop");
        }

        /// <summary>RemoveUnit takes the items out of the unit at once, but the drop events come after the call (seen 10-21), when the unit no longer has the item; the items are removed with the unit.</summary>
        public static void RemovedUnitDropsAfterTheCall(FakeWorld world)
        {
            ObjectTypes.Item(ObjectTypes.FourCC("I001"), () => new CarrierWatch());
            var handle = world.NewUnit(ObjectTypes.FourCC("H001"), hero: true);
            var hero = Unit.Of(handle)!;
            var watch = (CarrierWatch)Item.Of(world.NewItem(ObjectTypes.FourCC("I001")))!;
            hero.Give(watch);
            Expect.Same(hero, watch.Carrier, "carried before");

            world.DeleteUnit(handle);

            Expect.True(!watch.DropSeen, "no drop event inside the call");
            Expect.True(watch.Carrier == null, "the unit no longer has the item");

            world.Advance(0.1f);

            Expect.True(watch.DropSeen, "the drop event came after the call");
            Expect.True(watch.CarrierAtDrop == null, "the unit has no item at the event");
            Expect.Equal(0, world.ItemTypeId(watch.Handle), "the item is removed with the unit");
            world.Advance(0.3f);
            Expect.True(watch.IsGone, "the sweep finds the item gone");
        }

        /// <summary>Giving an item to a full inventory, or to a unit without one, answers false and fires no event; the item stays where it is (seen 10-21).</summary>
        public static void GiveFullInventory(FakeWorld world)
        {
            ObjectTypes.Item(ObjectTypes.FourCC("I001"), () => new CarrierWatch());
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            world.SetInventorySize(handle, 1);
            var unit = Unit.Of(handle)!;
            Expect.True(unit.Give(Item.Of(world.NewItem(ObjectTypes.FourCC("ofoo")))!), "the first item fits");
            var watch = (CarrierWatch)Item.Create(ObjectTypes.FourCC("I001"), 9.0f, 8.0f);
            world.InventoryCalls.Clear();

            Expect.True(!unit.Give(watch), "the full inventory answers false");

            Expect.True(watch.CarrierAtPickup == null && watch.Carrier == null, "no pick-up event");
            Expect.True(!watch.DropSeen, "no drop event");
            Expect.Equal(0, world.InventoryCalls.Count, "nothing changed");
            Expect.Near(9.0f, watch.X, "the item stays on the ground");

            var bare = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            world.SetInventorySize(bare, 0);
            Expect.True(!Unit.Of(bare)!.Give(watch), "a unit without an inventory answers false");
            Expect.True(watch.Carrier == null, "no carrier");
        }

        /// <summary>Giving an item the unit already holds fires a drop and then a pick-up, and the item keeps its slot (seen 10-21).</summary>
        public static void GiveHeldItemAgain(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("H001"), () => new TestHero());
            ObjectTypes.Item(ObjectTypes.FourCC("I001"), () => new TestOrb());
            var hero = Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), hero: true))!;
            var orb = Item.Of(world.NewItem(ObjectTypes.FourCC("I001")))!;
            hero.Give(orb);
            TestLog.Lines.Clear();

            Expect.True(hero.Give(orb), "the game accepts the item again");

            ExpectLog(new string[] { "glow dropped", "hero dropped orb", "tally dropped orb", "orb picked", "glow picked", "hero got orb", "tally got orb" }, "the log");
            Expect.Same(orb, hero.ItemIn(0), "the item keeps slot 0");
            Expect.True(hero.ItemIn(1) == null, "no second slot used");
            Expect.Same(hero, orb.Carrier, "still carried");
        }

        /// <summary>A freed slot is filled first (seen 10-21).</summary>
        public static void FreedSlotFirst(FakeWorld world)
        {
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var a = Item.Of(world.NewItem(ObjectTypes.FourCC("ofoo")))!;
            var b = Item.Of(world.NewItem(ObjectTypes.FourCC("ofoo")))!;
            var c = Item.Of(world.NewItem(ObjectTypes.FourCC("ofoo")))!;
            unit.Give(a);
            unit.Give(b);

            unit.Drop(a);
            unit.Give(c);

            Expect.Same(c, unit.ItemIn(0), "slot 0 is filled again");
            Expect.Same(b, unit.ItemIn(1), "slot 1");
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

        /// <summary>An item that records what its charges ask of the hooks, and reads the declared value as Item does.</summary>
        private sealed class RecordingItem : Item
        {
            /// <summary>The hook calls, in turn, as "read" or "declare", the field id, and the value declared.</summary>
            public List<string> Asked = new List<string>();

            /// <summary>Records the read, and reads the value as Item does.</summary>
            protected override T DeclaredValue<T>(string field)
            {
                Asked.Add("read " + field);
                return base.DeclaredValue<T>(field);
            }

            /// <summary>Records the declared value.</summary>
            protected override void Declare(string field, object value)
            {
                Asked.Add("declare " + field + " " + value);
            }
        }

        /// <summary>An item with no handle calls no native for its charges: a set goes to Declare and a read to DeclaredValue, which reads the values of the type its class is registered with; its default object reads them too.</summary>
        public static void SharedWithoutHandle(FakeWorld world)
        {
            var id = ObjectTypes.FourCC("I0rc");
            ObjectTypes.Item(id, () => new RecordingItem());
            world.SetTypeValue(id, "iuse", 3);
            var item = new RecordingItem();
            world.CallsThrow = true;

            item.Charges = 5;
            var charges = item.Charges;
            var declared = Item.Default<RecordingItem>().Charges;

            world.CallsThrow = false;
            Expect.Equal(3, charges, "the charges its type declares");
            Expect.Equal(3, declared, "the default's charges");
            Expect.Equal(2, item.Asked.Count, "hook calls");
            Expect.Equal("declare iuse 5", item.Asked[0], "the set");
            Expect.Equal("read iuse", item.Asked[1], "the read");
        }
    }
}
