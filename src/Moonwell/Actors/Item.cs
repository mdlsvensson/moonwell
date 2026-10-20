using System;
using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>An item of the game: an object of its declared class, which holds the components the class starts with.</summary>
    public class Item : Widget
    {
        /// <summary>The table behind ItemHooks, made on first use so that the class has no static initializer (see Unit.UnitHooks).</summary>
        private static HookTable? itemHooks;

        /// <summary>The reactions of items; declared here, so that an override in a subclass is found.</summary>
        internal static HookTable ItemHooks
        {
            get
            {
                if (itemHooks == null)
                {
                    itemHooks = new HookTable(typeof(Item), new string[] { "OnPickedUp", "OnDropped" }, new int[] { Hook.OnPickedUp, Hook.OnDropped });
                }

                return itemHooks;
            }
        }

        /// <summary>The bit mask of the reactions this class overrides, worked out when the item is taken in.</summary>
        internal int Mask;

        /// <summary>The components the class starts with, added in order when the item is taken in. Assigning a new list drops what a parent class listed.</summary>
        protected List<ItemComponent> Components { get; set; } = new List<ItemComponent>();

        /// <summary>The game's handle of the item.</summary>
        [LocalSafe]
        public item Handle { get; internal set; } = null!;

        /// <summary>The type id of the item, as the game read it when the item was taken in.</summary>
        [LocalSafe]
        public int TypeId { get; internal set; }

        /// <summary>The starting components, which the runtime adds in order when the item is taken in.</summary>
        internal List<ItemComponent> StartComponents => Components;

        /// <summary>The object of a game item: made when the item is first seen; null for null or for an item the game has removed.</summary>
        public static Item? Of(item? handle)
        {
            if (handle == null)
            {
                return null;
            }

            return Items.TakeIn(handle, null);
        }

        /// <summary>Creates an item of a type at a place; owner, when given, owns it.</summary>
        public static Item Create(int typeId, float x, float y, Owner? owner = null)
        {
            var handle = Runtime.World.SpawnItem(typeId, x, y);
            var made = Items.TakeIn(handle, owner)!;
            if (owner != null && made.Owner == null)
            {
                owner.Adopt(made);
            }

            return made;
        }

        /// <summary>Adds a component to the item, and returns it.</summary>
        public T Add<T>(T component)
            where T : ItemComponent => AddComponent(component);

        /// <summary>Removes the item from the game: it ends, which runs its OnLeave.</summary>
        public void Remove() => End();

        /// <summary>Runs when the item enters the game, after its starting components are added.</summary>
        protected virtual void OnEnter()
        {
        }

        /// <summary>Runs when the item leaves the game, before its components end.</summary>
        protected virtual void OnLeave()
        {
        }

        /// <summary>Runs OnEnter; Items calls it, since a map overrides the hook as protected.</summary>
        internal void CallEnter()
        {
            OnEnter();
        }

        /// <summary>Runs when the item is picked up by a unit, before the components see it.</summary>
        public virtual void OnPickedUp(Unit by)
        {
        }

        /// <summary>Runs when the item is dropped by a unit, before the components see it.</summary>
        public virtual void OnDropped(Unit by)
        {
        }

        /// <summary>Runs OnLeave first; the components then end, and the item leaves the game.</summary>
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

        /// <summary>Removes the item from the game unless the game has already, and forgets it.</summary>
        internal override void Finish()
        {
            if (!IsGone)
            {
                Runtime.World.DeleteItem(Handle);
                IsGone = true;
            }

            Items.Forget(this);
        }
    }
}
