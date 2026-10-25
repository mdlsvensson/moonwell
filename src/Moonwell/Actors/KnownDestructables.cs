using System;
using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>The destructables the runtime holds: the object of each game destructable, found by its handle, and the live ones in order.</summary>
    internal static class KnownDestructables
    {
        /// <summary>The object of each destructable the runtime holds, keyed by the game's handle; a lookup only.</summary>
        private static Dictionary<destructable, Destructable> known = new Dictionary<destructable, Destructable>();

        /// <summary>The destructables the runtime holds, in the order they were taken in.</summary>
        internal static OrderedSet<Destructable> Live = new OrderedSet<Destructable>();

        /// <summary>The object of a game destructable when the runtime holds it; null otherwise.</summary>
        internal static Destructable? Find(destructable handle)
        {
            if (known.TryGetValue(handle, out var found))
            {
                return found;
            }

            return null;
        }

        /// <summary>Takes a game destructable in: makes its object, adds its starting components, and runs OnCreated.</summary>
        internal static Destructable? TakeIn(destructable handle, Owner? owner)
        {
            var existing = Find(handle);
            if (existing != null)
            {
                return existing;
            }

            var typeId = Runtime.World.DestructableTypeId(handle);
            if (typeId == 0)
            {
                return null;
            }

            var maker = ObjectTypes.DestructableMaker(typeId);
            Destructable made;
            if (maker != null)
            {
                made = maker();
            }
            else
            {
                made = new Destructable();
            }

            made.Handle = handle;
            made.TypeId = typeId;
            made.X = Runtime.World.DestructableX(handle);
            made.Y = Runtime.World.DestructableY(handle);
            known[handle] = made;
            Live.Add(made);
            owner?.Adopt(made);
            made.Mask = Hooks.MaskOf(made, Destructable.DestructableHooks);
            Events.Need(made.Mask);
            made.WatchIfWanted(made.Mask);
            for (var i = 0; i < made.StartComponents.Count; i++)
            {
                made.Add(made.StartComponents[i]);
            }

            if (!made.HasEnded)
            {
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

        /// <summary>Forgets a destructable the game has removed, so that its handle makes no object again.</summary>
        internal static void Forget(Destructable destructable)
        {
            known.Remove(destructable.Handle);
            Live.Remove(destructable);
        }

        /// <summary>Forgets every destructable; Runtime.Reset calls it.</summary>
        internal static void Reset()
        {
            known.Clear();
            Live.Clear();
        }
    }
}
