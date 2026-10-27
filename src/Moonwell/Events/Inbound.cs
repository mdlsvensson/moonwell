using System;

namespace Moonwell
{
    /// <summary>The events the world delivers to the runtime, and what the runtime does with each.</summary>
    internal static class Inbound
    {
        /// <summary>The damage Unit.Damage is dealing, whose event keeps the damage it makes; null outside the call.</summary>
        private static Dealing? dealing;
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

        /// <summary>A unit enters the map: it is taken in when its type is registered, when a component wants to hear of entries, or once the interactions package tracks units (KnownUnits.EveryUnit).</summary>
        internal static void Entered(unit handle)
        {
            if (KnownUnits.Find(handle) != null)
            {
                return;
            }

            if (KnownUnits.EveryUnit || ObjectTypes.UnitMaker(Runtime.World.UnitTypeId(handle)) != null || Events.Wants(Hook.OnUnitCreated))
            {
                KnownUnits.TakeIn(handle, null);
            }
        }

        /// <summary>A unit picks up an item: the item's OnPickedUp and components, then the unit's OnItemPickedUp and components.</summary>
        internal static void PickedUp(unit u, item i)
        {
            if (KnownUnits.Find(u) == null && KnownItems.Find(i) == null && ObjectTypes.ItemMaker(Runtime.World.ItemTypeId(i)) == null)
            {
                return;
            }

            var by = Unit.Of(u);
            var picked = Item.Of(i);
            if (by == null || picked == null)
            {
                return;
            }

            picked.CarrierUnit = by;
            ItemPickedUp(picked, by);
            UnitItemPickedUp(by, picked);
        }

        /// <summary>A unit drops an item: the item's OnDropped and components, then the unit's OnItemDropped and components.</summary>
        internal static void Dropped(unit u, item i)
        {
            if (KnownUnits.Find(u) == null && KnownItems.Find(i) == null && ObjectTypes.ItemMaker(Runtime.World.ItemTypeId(i)) == null)
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

        /// <summary>A destructable dies: its type's OnDeath, then its components' OnDeath; one that is ending reacts to none, since the game fires the death event inside RemoveDestructable.</summary>
        internal static void DestructableDied(destructable d)
        {
            var dead = Destructable.Of(d);
            if (dead == null || dead.ending)
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

        /// <summary>A unit takes damage: the source's type and components deal it, the target's take it, then the map's and the players' reactions; returns the amount after them.</summary>
        internal static float Damaged(unit target, unit? source, float amount, bool isAttack)
        {
            if (KnownUnits.Find(target) == null && (source == null || KnownUnits.Find(source) == null) && !Events.Wants(Hook.OnUnitDamaged))
            {
                return amount;
            }

            var hit = Unit.Of(target);
            if (hit == null)
            {
                return amount;
            }

            Unit? from = source == null ? null : Unit.Of(source);

            // The game's event says no attack for damage from code even when dealt as one, so Unit.Damage's own flag counts.
            var deal = dealing != null && dealing.Damage == null && dealing.Target == target && dealing.Source == source ? dealing : null;
            var damage = new Damage(hit, from, amount, deal != null ? deal.IsAttack : isAttack);
            if (deal != null)
            {
                deal.Damage = damage;
                damage.Tag = deal.Tag;
            }

            Dispatch(damage);
            return damage.Amount;
        }

        /// <summary>Deals damage from code through the world, and returns the damage the reactions saw; one made here when none ran, null when the game dealt nothing.</summary>
        internal static Damage? Deal(Unit source, Unit target, float amount, DamageKind kind, AttackType attackType, bool isAttack, bool ranged, WeaponSound sound, object? tag)
        {
            var previous = dealing;
            var deal = new Dealing(source.Handle, target.Handle, isAttack, tag);
            dealing = deal;
            bool dealt;
            try
            {
                dealt = Runtime.World.DealDamage(source.Handle, target.Handle, amount, isAttack, ranged, TypeCodes.AttackCode(attackType), (int)kind, (int)sound);
            }
            finally
            {
                dealing = previous;
            }

            if (!dealt)
            {
                return null;
            }

            var damage = deal.Damage ?? new Damage(target, source, amount, isAttack);
            damage.Tag = tag;
            return damage;
        }

        /// <summary>A damage Unit.Damage deals: its handles, and the damage its event made.</summary>
        private sealed class Dealing
        {
            internal Dealing(unit source, unit target, bool isAttack, object? tag)
            {
                Source = source;
                Target = target;
                IsAttack = isAttack;
                Tag = tag;
            }

            internal bool IsAttack { get; }

            internal object? Tag { get; }

            internal unit Source { get; }

            internal unit Target { get; }

            internal Damage? Damage { get; set; }
        }

        /// <summary>Runs a damage's reactions: the source's type and components deal it, the target's take it, then the map's and the players' reactions; a damage package calls it with its own kind of damage.</summary>
        internal static void Dispatch(Damage damage)
        {
            var hit = damage.Target;
            var from = damage.Source;
            var hitSlot = hit.Player.Slot;
            var fromSlot = from == null ? -1 : from.Player.Slot;
            if (from != null)
            {
                DealtDamage(from, damage);
            }

            TookDamage(hit, damage);
            MapDamaged(Game.Instance, damage);
            var first = Player.Existing(hitSlot);
            if (first != null)
            {
                MapDamaged(first, damage);
            }

            if (from != null)
            {
                var second = Player.Existing(fromSlot);
                if (second != null && fromSlot != hitSlot)
                {
                    MapDamaged(second, damage);
                }
            }
        }

        /// <summary>A unit dies: its type's OnDeath and components, then the killer's OnKill and components, then the map's and the players' reactions, then the buffs a death ends end; a killer equal to the dying unit counts as none.</summary>
        internal static void Died(unit dying, unit? killer)
        {
            unit? slain = killer == dying ? null : killer;
            if (KnownUnits.Find(dying) == null && (slain == null || KnownUnits.Find(slain) == null) && !Events.Wants(Hook.OnUnitDied))
            {
                return;
            }

            var dead = Unit.Of(dying);
            if (dead == null)
            {
                return;
            }

            Unit? slayer = slain == null ? null : Unit.Of(slain);
            var deadSlot = dead.Player.Slot;
            var slayerSlot = slayer == null ? -1 : slayer.Player.Slot;
            Dying(dead, slayer);
            if (slayer != null)
            {
                Killing(slayer, dead);
            }

            MapDied(Game.Instance, dead, slayer);
            var first = Player.Existing(deadSlot);
            if (first != null)
            {
                MapDied(first, dead, slayer);
            }

            if (slayer != null)
            {
                var second = Player.Existing(slayerSlot);
                if (second != null && slayerSlot != deadSlot)
                {
                    MapDied(second, dead, slayer);
                }
            }

            Buff.EndOnDeath(dead);
        }

        /// <summary>A unit's spell takes effect: its previous cast ends, the ability and the caster see the new cast, then the map's and the caster's player's reactions run.</summary>
        internal static void SpellEffect(unit caster, int abilityId, unit? target, float x, float y)
        {
            if (KnownUnits.Find(caster) == null && ObjectTypes.AbilityMaker(abilityId) == null && !Events.Wants(Hook.OnUnitCast))
            {
                return;
            }

            var by = Unit.Of(caster);
            if (by == null)
            {
                return;
            }

            var casterSlot = by.Player.Slot;
            var running = by.ActiveCast;
            if (running != null)
            {
                running.End();
            }

            var ability = by.AbilityOf(abilityId);
            var cast = new Cast(ability, by, target == null ? null : Unit.Of(target), x, y, Runtime.World.AbilityLevel(caster, abilityId));
            ability.Adopt(cast);
            by.ActiveCast = cast;
            AbilityCast(ability, cast);
            CasterCast(by, cast);
            MapCast(Game.Instance, cast);
            var first = Player.Existing(casterSlot);
            if (first != null)
            {
                MapCast(first, cast);
            }
        }

        /// <summary>A unit's spell ends: the cast it is making ends, when that cast is live and is for this ability.</summary>
        internal static void SpellEnded(unit caster, int abilityId)
        {
            var ended = KnownUnits.Find(caster);
            if (ended == null)
            {
                return;
            }

            var cast = ended.ActiveCast;
            if (cast != null && !cast.HasEnded && cast.Ability.Id == abilityId)
            {
                cast.End();
            }
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

        /// <summary>Runs OnDealtDamage on a unit, then on its components, in add order.</summary>
        private static void DealtDamage(Unit from, Damage damage)
        {
            if (!from.HasEnded && (from.Mask & Hook.OnDealtDamage) != 0)
            {
                var previous = Owner.current;
                Owner.current = from;
                try
                {
                    from.OnDealtDamage(damage);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(from) + ".OnDealtDamage", error);
                }

                Owner.current = previous;
            }

            var count = from.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = from.ComponentAt(i) as UnitComponent;
                if (component == null || component.HasEnded || (component.Mask & Hook.OnDealtDamage) == 0)
                {
                    continue;
                }

                var previous = Owner.current;
                Owner.current = component;
                try
                {
                    component.OnDealtDamage(damage);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(component) + ".OnDealtDamage", error);
                }

                Owner.current = previous;
            }

            from.EndDispatch();
        }

        /// <summary>Runs OnDamaged on a unit, then on its components, in add order.</summary>
        private static void TookDamage(Unit hit, Damage damage)
        {
            if (!hit.HasEnded && (hit.Mask & Hook.OnDamaged) != 0)
            {
                var previous = Owner.current;
                Owner.current = hit;
                try
                {
                    hit.OnDamaged(damage);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(hit) + ".OnDamaged", error);
                }

                Owner.current = previous;
            }

            var count = hit.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = hit.ComponentAt(i) as UnitComponent;
                if (component == null || component.HasEnded || (component.Mask & Hook.OnDamaged) == 0)
                {
                    continue;
                }

                var previous = Owner.current;
                Owner.current = component;
                try
                {
                    component.OnDamaged(damage);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(component) + ".OnDamaged", error);
                }

                Owner.current = previous;
            }

            hit.EndDispatch();
        }

        /// <summary>Runs OnDeath on a unit, then on its components, in add order. The unit's own OnDeath runs with the game as the current owner (decision 64): death is not an end, and its corpse can go at any time, so what it makes, such as a respawn timer, must not end with it.</summary>
        private static void Dying(Unit dead, Unit? killer)
        {
            if (!dead.HasEnded && (dead.Mask & Hook.OnDeath) != 0)
            {
                var previous = Owner.current;
                Owner.current = Game.Instance;
                try
                {
                    dead.OnDeath(killer);
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
                var component = dead.ComponentAt(i) as UnitComponent;
                if (component == null || component.HasEnded || (component.Mask & Hook.OnDeath) == 0)
                {
                    continue;
                }

                var previous = Owner.current;
                Owner.current = component;
                try
                {
                    component.OnDeath(killer);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(component) + ".OnDeath", error);
                }

                Owner.current = previous;
            }

            dead.EndDispatch();
        }

        /// <summary>Runs OnKill on a unit, then on its components, in add order.</summary>
        private static void Killing(Unit slayer, Unit victim)
        {
            if (!slayer.HasEnded && (slayer.Mask & Hook.OnKill) != 0)
            {
                var previous = Owner.current;
                Owner.current = slayer;
                try
                {
                    slayer.OnKill(victim);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(slayer) + ".OnKill", error);
                }

                Owner.current = previous;
            }

            var count = slayer.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = slayer.ComponentAt(i) as UnitComponent;
                if (component == null || component.HasEnded || (component.Mask & Hook.OnKill) == 0)
                {
                    continue;
                }

                var previous = Owner.current;
                Owner.current = component;
                try
                {
                    component.OnKill(victim);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(component) + ".OnKill", error);
                }

                Owner.current = previous;
            }

            slayer.EndDispatch();
        }

        /// <summary>Runs OnUnitDamaged on the map components of an actor, in add order.</summary>
        private static void MapDamaged(Actor actor, Damage damage)
        {
            var count = actor.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = actor.ComponentAt(i) as MapComponent;
                if (component == null || component.HasEnded || (component.Mask & Hook.OnUnitDamaged) == 0)
                {
                    continue;
                }

                var previous = Owner.current;
                Owner.current = component;
                try
                {
                    component.OnUnitDamaged(damage);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(component) + ".OnUnitDamaged", error);
                }

                Owner.current = previous;
            }

            actor.EndDispatch();
        }

        /// <summary>Runs OnUnitDied on the map components of an actor, in add order.</summary>
        private static void MapDied(Actor actor, Unit dying, Unit? killer)
        {
            var count = actor.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = actor.ComponentAt(i) as MapComponent;
                if (component == null || component.HasEnded || (component.Mask & Hook.OnUnitDied) == 0)
                {
                    continue;
                }

                var previous = Owner.current;
                Owner.current = component;
                try
                {
                    component.OnUnitDied(dying, killer);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(component) + ".OnUnitDied", error);
                }

                Owner.current = previous;
            }

            actor.EndDispatch();
        }

        /// <summary>Runs OnCast on an ability, with the ability as the current owner (decision 63), unless the ability has ended or does not override it: what it makes runs on after the spell ends, and work that stops with a channel names the cast.</summary>
        private static void AbilityCast(Ability ability, Cast cast)
        {
            if (!ability.HasEnded && (ability.Mask & Hook.OnCast) != 0)
            {
                var previous = Owner.current;
                Owner.current = ability;
                try
                {
                    ability.OnCast(cast);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(ability) + ".OnCast", error);
                }

                Owner.current = previous;
            }
        }

        /// <summary>Runs OnCast on a unit, then on its components, in add order.</summary>
        private static void CasterCast(Unit caster, Cast cast)
        {
            if (!caster.HasEnded && (caster.Mask & Hook.OnCast) != 0)
            {
                var previous = Owner.current;
                Owner.current = caster;
                try
                {
                    caster.OnCast(cast);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(caster) + ".OnCast", error);
                }

                Owner.current = previous;
            }

            var count = caster.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = caster.ComponentAt(i) as UnitComponent;
                if (component == null || component.HasEnded || (component.Mask & Hook.OnCast) == 0)
                {
                    continue;
                }

                var previous = Owner.current;
                Owner.current = component;
                try
                {
                    component.OnCast(cast);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(component) + ".OnCast", error);
                }

                Owner.current = previous;
            }

            caster.EndDispatch();
        }

        /// <summary>A player leaves the game: the game's components react, then the leaving player's.</summary>
        internal static void Left(player p)
        {
            var leaving = Player.Of(p);
            if (leaving == null)
            {
                return;
            }

            MapLeft(Game.Instance, leaving);
            MapLeft(leaving, leaving);
        }

        /// <summary>Runs OnLeave on the map components of an actor, in add order.</summary>
        private static void MapLeft(Actor actor, Player leaving)
        {
            var count = actor.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = actor.ComponentAt(i) as MapComponent;
                if (component == null || component.HasEnded || (component.Mask & Hook.OnLeave) == 0)
                {
                    continue;
                }

                var previous = Owner.current;
                Owner.current = component;
                try
                {
                    component.OnLeave(leaving);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(component) + ".OnLeave", error);
                }

                Owner.current = previous;
            }

            actor.EndDispatch();
        }

        /// <summary>Runs OnUnitCast on the map components of an actor, in add order.</summary>
        private static void MapCast(Actor actor, Cast cast)
        {
            var count = actor.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = actor.ComponentAt(i) as MapComponent;
                if (component == null || component.HasEnded || (component.Mask & Hook.OnUnitCast) == 0)
                {
                    continue;
                }

                var previous = Owner.current;
                Owner.current = component;
                try
                {
                    component.OnUnitCast(cast);
                }
                catch (Exception error)
                {
                    Errors.Report(Names.NameOf(component) + ".OnUnitCast", error);
                }

                Owner.current = previous;
            }

            actor.EndDispatch();
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
