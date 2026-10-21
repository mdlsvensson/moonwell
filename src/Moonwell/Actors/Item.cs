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

        /// <summary>The charges last read or set.</summary>
        private int lastCharges;

        /// <summary>The game's player that owned the item when it was last read or set; kept for when the game no longer gives it.</summary>
        internal player OwningPlayer = null!;

        /// <summary>The x last read or set.</summary>
        private float lastX;

        /// <summary>The y last read or set.</summary>
        private float lastY;

        /// <summary>The unit that last picked the item up, as the pick-up events Moonwell has seen say; a candidate that Carrier checks with the game.</summary>
        internal Unit? CarrierUnit;

        /// <summary>The item's charges. Once the game has removed the item, the last value read or set, and setting does nothing.</summary>
        [LocalSafe]
        public int Charges
        {
            get
            {
                if (!IsGone)
                {
                    lastCharges = Runtime.World.ChargesOf(Handle);
                }

                return lastCharges;
            }

            set
            {
                if (!IsGone)
                {
                    lastCharges = value;
                    Runtime.World.SetCharges(Handle, value);
                }
            }
        }

        /// <summary>The player that owns the item; setting it leaves the item's colour as it is. Once the game has removed the item, the last player read or set (the one it had when Moonwell took it in, if none); the neutral passive player when the game gives none. Setting does nothing then.</summary>
        [LocalSafe]
        public Player Player
        {
            get
            {
                if (!IsGone)
                {
                    var now = Runtime.World.ItemOwnerOf(Handle);
                    if (now != null)
                    {
                        OwningPlayer = now;
                    }
                }

                return Player.Of(OwningPlayer) ?? Player.NeutralPassive;
            }

            set
            {
                if (!IsGone)
                {
                    OwningPlayer = value.Handle;
                    Runtime.World.SetItemOwner(Handle, value.Handle);
                }
            }
        }

        /// <summary>The x of the item. Once the game has removed it, the last x read or set.</summary>
        [LocalSafe]
        public float X
        {
            get
            {
                if (!IsGone)
                {
                    lastX = Runtime.World.ItemX(Handle);
                }

                return lastX;
            }
        }

        /// <summary>The y of the item. Once the game has removed it, the last y read or set.</summary>
        [LocalSafe]
        public float Y
        {
            get
            {
                if (!IsGone)
                {
                    lastY = Runtime.World.ItemY(Handle);
                }

                return lastY;
            }
        }

        /// <summary>Moves the item to a place (SetItemPosition). Does nothing once the item is gone.</summary>
        public void MoveTo(float x, float y)
        {
            if (!IsGone)
            {
                lastX = x;
                lastY = y;
                Runtime.World.MoveItem(Handle, x, y);
            }
        }

        /// <summary>The unit that carries the item: the last unit seen picking it up, as long as the game says it still has the item (UnitHasItem), so a drop the game fires when an item only moves between slots does not make it uncarried. Null for an item picked up before Moonwell saw it (Moonwell does not look through inventories), and once the item or the carrier is gone. Inside OnDropped it is what the game answers at that moment: null when the item is already out of the inventory.</summary>
        [LocalSafe]
        public Unit? Carrier
        {
            get
            {
                var by = CarrierUnit;
                if (IsGone || by == null || by.IsGone || !Runtime.World.CarriesItem(by.Handle, Handle))
                {
                    return null;
                }

                return by;
            }
        }

        /// <summary>Whether a unit carries the item; see Carrier.</summary>
        [LocalSafe]
        public bool IsCarried => Carrier != null;

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
