using System;
using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>The items the runtime holds: the object of each game item, found by its handle, and the live ones in order.</summary>
    internal static class KnownItems
    {
        /// <summary>The object of each item the runtime holds, keyed by the game's handle; a lookup only.</summary>
        private static Dictionary<item, Item> known = new Dictionary<item, Item>();

        /// <summary>The items the runtime holds, in the order they were taken in.</summary>
        internal static OrderedSet<Item> Live = new OrderedSet<Item>();

        /// <summary>The object of a game item when the runtime holds it; null otherwise.</summary>
        internal static Item? Find(item handle)
        {
            if (known.TryGetValue(handle, out var found))
            {
                return found;
            }

            return null;
        }

        /// <summary>Takes a game item in: makes its object, adds its starting components, and runs OnCreated.</summary>
        internal static Item? TakeIn(item handle, Owner? owner)
        {
            var existing = Find(handle);
            if (existing != null)
            {
                return existing;
            }

            var typeId = Runtime.World.ItemTypeId(handle);
            if (typeId == 0)
            {
                return null;
            }

            var maker = ObjectTypes.ItemMaker(typeId);
            Item made;
            if (maker != null)
            {
                made = maker();
            }
            else
            {
                made = new Item();
            }

            made.Handle = handle;
            made.TypeId = typeId;
            made.OwningPlayer = Runtime.World.ItemOwnerOf(handle);
            known[handle] = made;
            Live.Add(made);
            owner?.Adopt(made);
            made.Mask = Hooks.MaskOf(made, Item.ItemHooks);
            Events.Need(made.Mask);
            for (var i = 0; i < made.StartComponents.Count; i++)
            {
                made.Add(made.StartComponents[i]);
            }

            // From here it has come in: its OnLeave and the ending listeners run when it ends. One that ended before (an
            // owner that has ended ends it at once) runs neither OnCreated nor OnLeave.
            if (!made.HasEnded)
            {
                made.entered = true;
                var previous = Owner.current;
                Owner.current = made;
                try
                {
                    made.CallEnter();
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(made) + ".OnCreated", error);
                }

                Owner.current = previous;
            }

            if (!made.HasEnded)
            {
                Hooks.CallTakenIn(made);
            }

            return made;
        }

        /// <summary>Forgets an item the game has removed, so that its handle makes no object again.</summary>
        internal static void Forget(Item item)
        {
            known.Remove(item.Handle);
            Live.Remove(item);
        }

        /// <summary>Forgets every item; Runtime.Reset calls it.</summary>
        internal static void Reset()
        {
            known.Clear();
            Live.Clear();
        }
    }
}
