namespace Moonwell
{
    /// <summary>A component on an item, which reacts to the item's events.</summary>
    public abstract class ItemComponent : Component
    {
        /// <summary>The table behind ComponentHooks, made on first use so that the class has no static initializer (see Unit.UnitHooks).</summary>
        private static HookTable? componentHooks;

        /// <summary>The reactions of item components, named as Item's are.</summary>
        internal static HookTable ComponentHooks
        {
            get
            {
                if (componentHooks == null)
                {
                    componentHooks = new HookTable(typeof(ItemComponent), new string[] { "OnPickedUp", "OnDropped" }, new int[] { Hook.OnPickedUp, Hook.OnDropped });
                }

                return componentHooks;
            }
        }

        /// <summary>The item this component is on.</summary>
        [LocalSafe]
        public Item Item => (Item)Actor;

        /// <summary>The reactions this kind of component declares.</summary>
        internal override HookTable Table => ComponentHooks;

        /// <summary>Runs when the item is picked up by a unit.</summary>
        public virtual void OnPickedUp(Unit by)
        {
        }

        /// <summary>Runs when the item is dropped by a unit.</summary>
        public virtual void OnDropped(Unit by)
        {
        }
    }
}
