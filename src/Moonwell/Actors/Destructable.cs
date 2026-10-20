using System;
using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>A destructable of the game: an object of its declared class, which holds the components the class starts with.</summary>
    public class Destructable : Widget
    {
        /// <summary>The table behind DestructableHooks, made on first use so that the class has no static initializer (see Unit.UnitHooks).</summary>
        private static HookTable? destructableHooks;

        /// <summary>The reactions of destructables; declared here, so that an override in a subclass is found.</summary>
        internal static HookTable DestructableHooks
        {
            get
            {
                if (destructableHooks == null)
                {
                    destructableHooks = new HookTable(typeof(Destructable), new string[] { "OnDeath" }, new int[] { Hook.OnDeath });
                }

                return destructableHooks;
            }
        }

        /// <summary>The bit mask of the reactions this class overrides, worked out when the destructable is taken in.</summary>
        internal int Mask;

        /// <summary>Whether the world has been asked to deliver this destructable's death, once.</summary>
        internal bool watched;

        /// <summary>The components the class starts with, added in order when the destructable is taken in. Assigning a new list drops what a parent class listed.</summary>
        protected List<DestructableComponent> Components { get; set; } = new List<DestructableComponent>();

        /// <summary>The game's handle of the destructable.</summary>
        [LocalSafe]
        public destructable Handle { get; internal set; } = null!;

        /// <summary>The type id of the destructable, as the game read it when it was taken in.</summary>
        [LocalSafe]
        public int TypeId { get; internal set; }

        /// <summary>The starting components, which the runtime adds in order when the destructable is taken in.</summary>
        internal List<DestructableComponent> StartComponents => Components;

        /// <summary>The object of a game destructable: made when it is first seen; null for null or for one the game has removed.</summary>
        public static Destructable? Of(destructable? handle)
        {
            if (handle == null)
            {
                return null;
            }

            return Destructables.TakeIn(handle, null);
        }

        /// <summary>Adds a component to the destructable, and returns it.</summary>
        public T Add<T>(T component)
            where T : DestructableComponent
        {
            var added = AddComponent(component);
            WatchIfWanted(Mask | added.Mask);
            return added;
        }

        /// <summary>Removes the destructable from the game: it ends, which runs its OnLeave.</summary>
        public void Remove() => End();

        /// <summary>Runs when the destructable enters the game, after its starting components are added.</summary>
        protected virtual void OnEnter()
        {
        }

        /// <summary>Runs when the destructable leaves the game, before its components end.</summary>
        protected virtual void OnLeave()
        {
        }

        /// <summary>Runs OnEnter; Destructables calls it, since a map overrides the hook as protected.</summary>
        internal void CallEnter()
        {
            OnEnter();
        }

        /// <summary>Runs when the destructable dies, before the components see it.</summary>
        public virtual void OnDeath()
        {
        }

        /// <summary>Asks the world to deliver this destructable's death, once, when a mask has the death bit.</summary>
        internal void WatchIfWanted(int mask)
        {
            if ((mask & Hook.OnDeath) != 0 && !watched && !HasEnded)
            {
                Runtime.World.WatchDeath(Handle);
                watched = true;
            }
        }

        /// <summary>Runs OnLeave first; the components then end, and the destructable leaves the game.</summary>
        protected sealed override void OnEnd()
        {
            var previous = Owner.current;
            Owner.current = this;
            try
            {
                OnLeave();
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(this) + ".OnLeave", error);
            }

            Owner.current = previous;
        }

        /// <summary>Removes the destructable from the game unless the game has already, and forgets it.</summary>
        internal override void Finish()
        {
            if (!IsGone)
            {
                Runtime.World.DeleteDestructable(Handle);
                IsGone = true;
            }

            Destructables.Forget(this);
        }
    }
}
