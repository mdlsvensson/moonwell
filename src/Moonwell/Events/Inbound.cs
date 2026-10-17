using System;

namespace Moonwell
{
    /// <summary>The events the world delivers to the runtime, and what the runtime does with each.</summary>
    internal static class Inbound
    {
        /// <summary>Runs OnGameStart on the game's components, then on each existing player's, in slot order.</summary>
        internal static void GameStarted()
        {
            StartComponents(Game.Instance);
            for (var slot = 0; slot < Player.MaxSlots; slot++)
            {
                var existing = Player.Existing(slot);
                if (existing != null)
                {
                    StartComponents(existing);
                }
            }
        }

        /// <summary>A unit enters the map: it is taken in when its type is registered, or when a component wants to hear of entries.</summary>
        internal static void Entered(unit handle)
        {
            if (Units.Find(handle) != null)
            {
                return;
            }

            if (ObjectTypes.UnitMaker(Runtime.World.UnitTypeId(handle)) != null || Events.Wants(Hook.OnUnitEnter))
            {
                Units.TakeIn(handle, null);
            }
        }

        /// <summary>A unit picks up an item: the item's OnPickedUp and components, then the unit's OnItemPickedUp and components.</summary>
        internal static void PickedUp(unit u, item i)
        {
            if (Units.Find(u) == null && Items.Find(i) == null && ObjectTypes.ItemMaker(Runtime.World.ItemTypeId(i)) == null)
            {
                return;
            }

            var by = Unit.Of(u);
            var picked = Item.Of(i);
            if (by == null || picked == null)
            {
                return;
            }

            ItemPickedUp(picked, by);
            UnitItemPickedUp(by, picked);
        }

        /// <summary>A unit drops an item: the item's OnDropped and components, then the unit's OnItemDropped and components.</summary>
        internal static void Dropped(unit u, item i)
        {
            if (Units.Find(u) == null && Items.Find(i) == null && ObjectTypes.ItemMaker(Runtime.World.ItemTypeId(i)) == null)
            {
                return;
            }

            var by = Unit.Of(u);
            var dropped = Item.Of(i);
            if (by == null || dropped == null)
            {
                return;
            }

            ItemDropped(dropped, by);
            UnitItemDropped(by, dropped);
        }

        /// <summary>A destructable dies: its type's OnDeath, then its components' OnDeath.</summary>
        internal static void DestructableDied(destructable d)
        {
            var dead = Destructable.Of(d);
            if (dead == null)
            {
                return;
            }

            if (!dead.HasEnded && (dead.Mask & Hook.OnDeath) != 0)
            {
                var previous = Owner.current;
                Owner.current = dead;
                try
                {
                    dead.OnDeath();
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(dead) + ".OnDeath", error);
                }

                Owner.current = previous;
            }

            var count = dead.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = dead.ComponentAt(i) as DestructableComponent;
                if (component == null || component.HasEnded || (component.Mask & Hook.OnDeath) == 0)
                {
                    continue;
                }

                var previous = Owner.current;
                Owner.current = component;
                try
                {
                    component.OnDeath();
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(component) + ".OnDeath", error);
                }

                Owner.current = previous;
            }

            dead.EndDispatch();
        }

        /// <summary>Runs OnPickedUp on an item, then on its components, in add order.</summary>
        private static void ItemPickedUp(Item item, Unit by)
        {
            if (!item.HasEnded && (item.Mask & Hook.OnPickedUp) != 0)
            {
                var previous = Owner.current;
                Owner.current = item;
                try
                {
                    item.OnPickedUp(by);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(item) + ".OnPickedUp", error);
                }

                Owner.current = previous;
            }

            var count = item.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = item.ComponentAt(i) as ItemComponent;
                if (component == null || component.HasEnded || (component.Mask & Hook.OnPickedUp) == 0)
                {
                    continue;
                }

                var previous = Owner.current;
                Owner.current = component;
                try
                {
                    component.OnPickedUp(by);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(component) + ".OnPickedUp", error);
                }

                Owner.current = previous;
            }

            item.EndDispatch();
        }

        /// <summary>Runs OnItemPickedUp on a unit, then on its components, in add order.</summary>
        private static void UnitItemPickedUp(Unit unit, Item item)
        {
            if (!unit.HasEnded && (unit.Mask & Hook.OnItemPickedUp) != 0)
            {
                var previous = Owner.current;
                Owner.current = unit;
                try
                {
                    unit.OnItemPickedUp(item);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(unit) + ".OnItemPickedUp", error);
                }

                Owner.current = previous;
            }

            var count = unit.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = unit.ComponentAt(i) as UnitComponent;
                if (component == null || component.HasEnded || (component.Mask & Hook.OnItemPickedUp) == 0)
                {
                    continue;
                }

                var previous = Owner.current;
                Owner.current = component;
                try
                {
                    component.OnItemPickedUp(item);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(component) + ".OnItemPickedUp", error);
                }

                Owner.current = previous;
            }

            unit.EndDispatch();
        }

        /// <summary>Runs OnDropped on an item, then on its components, in add order.</summary>
        private static void ItemDropped(Item item, Unit by)
        {
            if (!item.HasEnded && (item.Mask & Hook.OnDropped) != 0)
            {
                var previous = Owner.current;
                Owner.current = item;
                try
                {
                    item.OnDropped(by);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(item) + ".OnDropped", error);
                }

                Owner.current = previous;
            }

            var count = item.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = item.ComponentAt(i) as ItemComponent;
                if (component == null || component.HasEnded || (component.Mask & Hook.OnDropped) == 0)
                {
                    continue;
                }

                var previous = Owner.current;
                Owner.current = component;
                try
                {
                    component.OnDropped(by);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(component) + ".OnDropped", error);
                }

                Owner.current = previous;
            }

            item.EndDispatch();
        }

        /// <summary>Runs OnItemDropped on a unit, then on its components, in add order.</summary>
        private static void UnitItemDropped(Unit unit, Item item)
        {
            if (!unit.HasEnded && (unit.Mask & Hook.OnItemDropped) != 0)
            {
                var previous = Owner.current;
                Owner.current = unit;
                try
                {
                    unit.OnItemDropped(item);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(unit) + ".OnItemDropped", error);
                }

                Owner.current = previous;
            }

            var count = unit.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = unit.ComponentAt(i) as UnitComponent;
                if (component == null || component.HasEnded || (component.Mask & Hook.OnItemDropped) == 0)
                {
                    continue;
                }

                var previous = Owner.current;
                Owner.current = component;
                try
                {
                    component.OnItemDropped(item);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(component) + ".OnItemDropped", error);
                }

                Owner.current = previous;
            }

            unit.EndDispatch();
        }

        /// <summary>Runs OnGameStart on the map components of an actor, in add order.</summary>
        private static void StartComponents(Actor actor)
        {
            var count = actor.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = actor.ComponentAt(i) as MapComponent;
                if (component == null || component.HasEnded || (component.Mask & Hook.OnGameStart) == 0)
                {
                    continue;
                }

                var previous = Owner.current;
                Owner.current = component;
                try
                {
                    component.OnGameStart();
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(component) + ".OnGameStart", error);
                }

                Owner.current = previous;
            }

            actor.EndDispatch();
        }
    }
}
