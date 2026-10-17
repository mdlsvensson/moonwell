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

        /// <summary>A registered item that overrides OnPickedUp listens for pick-ups, and not for drops until a drop hook is overridden.</summary>
        public static void Listens(FakeWorld world)
        {
            ObjectTypes.Item(ObjectTypes.FourCC("I001"), () => new TestOrb());
            Expect.True(world.Listening.Contains(GameEvent.PickUp), "PickUp is listened for");
            Expect.True(!world.Listening.Contains(GameEvent.Drop), "Drop is not listened for");
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
    }
}
