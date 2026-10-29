using System;
using System.Collections.Generic;
using System.ComponentModel;

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

        /// <summary>The default object of the item's type, as Unreal's class default object: the values its class declares (a type no class of the map's declares, its values as World Editor made them, standard where World Editor left them), the same whatever this item's are now and once it is gone. One per type, shared by its items, with no handle; it is gone, so its live actions do nothing.</summary>
        [LocalSafe]
        public Item Default => ObjectTypes.DefaultOf(this, DeclaredTypeId);

        /// <summary>The id of the type whose declared values the item reads: its type id, else, while its constructor runs, the id its class is registered with, else 0.</summary>
        internal int DeclaredTypeId => TypeId != 0 ? TypeId : ObjectTypes.IdOfClass(this);

        /// <summary>The value the item's type declares for a field, which a shared member reads when the item has no handle (in its constructor, and on a default object). Here, the value the build wrote for the map; the standard classes read their own declared values on .NET.</summary>
        [EditorBrowsable(EditorBrowsableState.Never)]
        protected virtual T DeclaredValue<T>(string field)
        {
            // The build has no world; the classes it reads override this.
            if (Runtime.World == null)
            {
                return default(T)!;
            }

            var value = Runtime.World.TypeValue(DeclaredTypeId, field);
            return value == null ? default(T)! : (T)value;
        }

        /// <summary>Declares the value of a field for the item's class, which a shared member does when it is set with no handle. Here, nothing: in the game the values are the build's; the standard classes keep them on .NET.</summary>
        [EditorBrowsable(EditorBrowsableState.Never)]
        protected virtual void Declare(string field, object value)
        {
        }

        // The instance fields are internal, not private, as a map's subclass may name a field of its own the same (see Scope).

        /// <summary>The charges last read or set.</summary>
        internal int lastCharges;

        /// <summary>The game's player that owned the item when it was last read or set; kept for when the game no longer gives it.</summary>
        internal player OwningPlayer = null!;

        /// <summary>The x last read or set.</summary>
        internal float lastX = 0.0f;

        /// <summary>The y last read or set.</summary>
        internal float lastY = 0.0f;

        /// <summary>The unit that last picked the item up, as the pick-up events Moonwell has seen say; a candidate that Carrier checks with the game.</summary>
        internal Unit? CarrierUnit;

        /// <summary>Whether a getter that read 0 read it from an item the game has removed: the game gives 0 for everything then, and the type id 0, before the sweep finds the item. The getter then keeps the last value instead of the 0.</summary>
        internal bool RemovedByGame()
        {
            return Runtime.World.ItemTypeId(Handle) == 0;
        }

        /// <summary>The item's charges (what the type gives: a healing potion 1, Claws of Attack 0). Any value is taken at once; 0 does not remove the item, it stays where it is, on the ground or in an inventory, and a carried potion of 0 charges can still be used. Once the game has removed the item, the last value read or set, and setting does nothing. In a declared class's constructor, the charges every item of the class starts with (iuse); on a default object, its type's.</summary>
        [LocalSafe]
        public int Charges
        {
            get
            {
                if (Handle == null)
                {
                    return DeclaredValue<int>("iuse");
                }

                if (IsGone)
                {
                    return lastCharges;
                }

                var now = Runtime.World.ChargesOf(Handle);
                if (now == 0 && RemovedByGame())
                {
                    return lastCharges;
                }

                if (Local.Depth == 0)
                {
                    lastCharges = now;
                }

                return now;
            }

            set
            {
                if (Handle == null)
                {
                    Declare("iuse", value);
                    return;
                }

                if (!IsGone)
                {
                    lastCharges = value;
                    Runtime.World.SetCharges(Handle, value);
                }
            }
        }

        /// <summary>The player that owns the item; a new item belongs to the neutral passive player. Picking the item up or dropping it does not change the owner, so this is not the carrier's player. Setting it takes effect at once and leaves the item's colour as it is. Once the game has removed the item, the last player read or set (the one it had when Moonwell took it in, if none); the neutral passive player when the game gives none. Setting does nothing then.</summary>
        [LocalSafe]
        public Player Owner
        {
            get
            {
                if (!IsGone)
                {
                    var now = Runtime.World.ItemOwnerOf(Handle);
                    if (now != null)
                    {
                        if (Local.Depth != 0)
                        {
                            return Player.Of(now) ?? Player.NeutralPassive;
                        }

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

        /// <summary>The x of the item. While a unit carries it, this is the x where it was picked up, not the carrier's: read Carrier's X for that; a drop puts the item at the unit's place. Once the game has removed it, the last x read or set.</summary>
        [LocalSafe]
        public float X
        {
            get
            {
                if (IsGone)
                {
                    return lastX;
                }

                var now = Runtime.World.ItemX(Handle);
                if (now == 0.0f && RemovedByGame())
                {
                    return lastX;
                }

                if (Local.Depth == 0)
                {
                    lastX = now;
                }

                return now;
            }
        }

        /// <summary>The y of the item. While a unit carries it, this is the y where it was picked up, not the carrier's: read Carrier's Y for that; a drop puts the item at the unit's place. Once the game has removed it, the last y read or set.</summary>
        [LocalSafe]
        public float Y
        {
            get
            {
                if (IsGone)
                {
                    return lastY;
                }

                var now = Runtime.World.ItemY(Handle);
                if (now == 0.0f && RemovedByGame())
                {
                    return lastY;
                }

                if (Local.Depth == 0)
                {
                    lastY = now;
                }

                return now;
            }
        }

        /// <summary>Moves the item to a place (SetItemPosition), through the game's pathing: a place on a tree or a farm becomes a pathable spot nearby, and past the edge it stops inside the playable area (seen), so X and Y may read other than what was given. Does nothing once the item is gone.</summary>
        public void MoveTo(float x, float y)
        {
            if (!IsGone)
            {
                Runtime.World.MoveItem(Handle, x, y);

                // The game snaps the place, so the last place is the one it gives back, not the one asked. An item the
                // game has removed reads 0 (the sweep has not found it yet): that keeps the last place.
                var x2 = Runtime.World.ItemX(Handle);
                var y2 = Runtime.World.ItemY(Handle);
                if (!(x2 == 0.0f && y2 == 0.0f && RemovedByGame()))
                {
                    lastX = x2;
                    lastY = y2;
                }
            }
        }

        /// <summary>The unit that carries the item: the last unit seen picking it up, as long as the game says it still has the item (UnitHasItem), so a drop the game fires when an item only moves between slots does not make it uncarried. Null for an item picked up before Moonwell saw it (Moonwell does not look through inventories), for an item in a bag or equipped (the 3.0 extended inventory: the unit does not count it as held), and once the item or the carrier is gone. Inside OnDropped it is still the unit, since the game takes the item out of the slot after the drop event, and it is null after the call. The exception is a drop after a unit was removed from the game (RemoveUnit): the game fires it after the call, when the unit no longer has the item, so Carrier is already null in OnDropped. After a hero dies it stays the hero, which keeps its items; a unit that is not a hero drops them when it dies, so it is no longer the carrier.</summary>
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

            return KnownItems.TakeIn(handle, null);
        }

        /// <summary>Creates an item of a type at a place, in a scope, which ends it when it ends. Throws a MoonwellException naming the id when the game makes no item of the type.</summary>
        public static Item Create(int typeId, float x, float y, Scope scope)
        {
            var handle = Runtime.World.SpawnItem(typeId, x, y);
            if (handle == null)
            {
                throw new MoonwellException("Item.Create: the game made no item of type " + Names.IdText(typeId));
            }

            var made = KnownItems.TakeIn(handle, scope)!;
            if (made.Parent == null)
            {
                scope.Adopt(made);
            }

            return made;
        }

        /// <summary>Creates an item of a registered class at a place, in a scope, which ends it when it ends.</summary>
        public static T Create<T>(float x, float y, Scope scope)
            where T : Item
        {
            return (T)Create(ObjectTypes.IdOf<T>(), x, y, scope);
        }

        /// <summary>Adds a component to the item, and returns it.</summary>
        public T Add<T>(T component)
            where T : ItemComponent => AddComponent(component);

        /// <summary>Removes the item from the game: it ends, which runs its OnLeave. Does nothing on a default object, which is not in the game.</summary>
        public void Remove()
        {
            if (Handle != null)
            {
                End();
            }
        }

        /// <summary>Runs once, when the item comes into the game, after its starting components are added.</summary>
        protected virtual void OnCreated()
        {
        }

        /// <summary>Runs when the item leaves the game, before its components end; HasEnded is still false here. It runs once for each OnCreated: one made for an owner that had already ended, which ends at once, runs neither.</summary>
        protected virtual void OnLeave()
        {
        }

        /// <summary>Runs OnCreated; Items calls it, since a map overrides the hook as protected.</summary>
        internal void CallEnter()
        {
            OnCreated();
        }

        /// <summary>Runs when the item is picked up by a unit, before the components see it.</summary>
        /// <param name="carrierUnit">The unit that picked the item up.</param>
        public virtual void OnPickedUp(Unit carrierUnit)
        {
        }

        /// <summary>Runs when the item is dropped by a unit, before the components see it. Inside the call that drops it (Drop, UnitRemoveItem) Carrier is still the unit; for the drops the game fires after RemoveUnit it is already null (see Unit.Drop). Not run when a unit that is not a hero dies and drops its items.</summary>
        /// <param name="carrierUnit">The unit that dropped the item.</param>
        public virtual void OnDropped(Unit carrierUnit)
        {
        }

        /// <summary>Tells the interactions package, if there is one; runs OnLeave; the components then end, and the item leaves the game. One that never came in, as one made for an owner that had ended, runs neither, as it ran no OnCreated.</summary>
        protected sealed override void OnEnd()
        {
            if (!entered)
            {
                return;
            }

            Hooks.CallEnding(this);
            try
            {
                OnLeave();
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(this) + ".OnLeave", error);
            }
        }

        /// <summary>Removes the item from the game unless the game has already, and forgets it.</summary>
        internal override void Finish()
        {
            if (!IsGone)
            {
                Runtime.World.DeleteItem(Handle);
                IsGone = true;
            }

            KnownItems.Forget(this);
        }
    }
}
