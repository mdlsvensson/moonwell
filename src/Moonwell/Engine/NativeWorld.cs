using System;
using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell.Engine
{
    /// <summary>The game as the runtime sees it, through the game's natives; the only class that calls them.</summary>
    internal sealed class NativeWorld : IWorld
    {
        /// <summary>How many lines the log keeps; the file holds the last ones.</summary>
        private const int LogLines = 200;

        /// <summary>The clock: its elapsed time is the game time the runtime has run for.</summary>
        private readonly timer clock;

        /// <summary>The timer that calls Runtime.Wake when the next wake is due.</summary>
        private readonly timer wake;

        /// <summary>The trigger that sees each destructable die, shared by every destructable watched.</summary>
        private readonly trigger deathTrigger;

        /// <summary>The last lines of the log, written to the log file each time a line is added.</summary>
        private readonly List<string> lines = new List<string>();

        /// <summary>The items the last ItemsInWorld call has found.</summary>
        private List<item> foundItems = new List<item>();

        /// <summary>The destructables the last DestructablesInWorld call has found.</summary>
        private List<destructable> foundDestructables = new List<destructable>();

        /// <summary>Starts the clock and the wake timer, and listens for units entering the map.</summary>
        public NativeWorld()
        {
            clock = Natives.CreateTimer();
            Natives.TimerStart(clock, 604800f, false, ClockDone);
            wake = Natives.CreateTimer();

            var region = Natives.CreateRegion();
            Natives.RegionAddRect(region, Natives.GetWorldBounds());
            var enter = Natives.CreateTrigger();
            Natives.TriggerRegisterEnterRegion(enter, region, null);
            Natives.TriggerAddAction(enter, EnterAction);

            deathTrigger = Natives.CreateTrigger();
            Natives.TriggerAddAction(deathTrigger, DestructableDeathAction);
        }

        /// <summary>Game seconds since the runtime started.</summary>
        public float Now()
        {
            return Natives.TimerGetElapsed(clock);
        }

        /// <summary>Calls Runtime.Wake once after the seconds; a later request replaces it.</summary>
        public void WakeIn(float seconds)
        {
            Natives.TimerStart(wake, seconds < 0f ? 0f : seconds, false, Runtime.Wake);
        }

        /// <summary>Starts delivering an event to Inbound.</summary>
        public void Listen(GameEvent gameEvent)
        {
            switch (gameEvent)
            {
                case GameEvent.Damage:
                    Register(Natives.EVENT_PLAYER_UNIT_DAMAGED, DamageAction);
                    break;
                case GameEvent.Death:
                    Register(Natives.EVENT_PLAYER_UNIT_DEATH, DeathAction);
                    break;
                case GameEvent.Spell:
                    Register(Natives.EVENT_PLAYER_UNIT_SPELL_EFFECT, SpellEffectAction);
                    Register(Natives.EVENT_PLAYER_UNIT_SPELL_ENDCAST, SpellEndedAction);
                    break;
                case GameEvent.PickUp:
                    Register(Natives.EVENT_PLAYER_UNIT_PICKUP_ITEM, PickUpAction);
                    break;
                case GameEvent.Drop:
                    Register(Natives.EVENT_PLAYER_UNIT_DROP_ITEM, DropAction);
                    break;
            }
        }

        /// <summary>Delivers a destructable's death to Inbound.</summary>
        public void WatchDeath(destructable destructable)
        {
            Natives.TriggerRegisterDeathEvent(deathTrigger, destructable);
        }

        /// <summary>Shows a line on the screen of the local player.</summary>
        public void Print(string text)
        {
            Natives.DisplayTimedTextToPlayer(Natives.GetLocalPlayer(), 0f, 0f, 30f, text);
        }

        /// <summary>Appends a line to the log, and rewrites the log file with its last lines.</summary>
        public void Log(string text)
        {
            lines.Add(text);
            if (lines.Count > LogLines)
            {
                lines.RemoveAt(0);
            }

            Natives.PreloadGenClear();
            Natives.PreloadGenStart();
            foreach (var line in lines)
            {
                Natives.Preload("\")\n" + line.Replace("\"", "'") + "\n//");
            }

            Natives.PreloadGenEnd("Moonwell\\log.txt");
        }

        /// <summary>The player in a slot.</summary>
        public player PlayerAt(int slot)
        {
            return Natives.Player(slot);
        }

        /// <summary>The slot of a player.</summary>
        public int SlotOf(player player)
        {
            return Natives.GetPlayerId(player);
        }

        /// <summary>Every unit in the world.</summary>
        public List<unit> UnitsInWorld()
        {
            var group = Natives.CreateGroup();
            Natives.GroupEnumUnitsInRect(group, Natives.GetWorldBounds(), null);
            var units = new List<unit>();
            var count = Natives.BlzGroupGetSize(group);
            for (var i = 0; i < count; i++)
            {
                units.Add(Natives.BlzGroupUnitAt(group, i));
            }

            Natives.DestroyGroup(group);
            return units;
        }

        /// <summary>Every item in the world.</summary>
        public List<item> ItemsInWorld()
        {
            foundItems = new List<item>();
            Natives.EnumItemsInRect(Natives.GetWorldBounds(), null, CollectItem);
            return foundItems;
        }

        /// <summary>Every destructable in the world.</summary>
        public List<destructable> DestructablesInWorld()
        {
            foundDestructables = new List<destructable>();
            Natives.EnumDestructablesInRect(Natives.GetWorldBounds(), null, CollectDestructable);
            return foundDestructables;
        }

        /// <summary>Creates a unit of a type for an owner at a place and facing.</summary>
        public unit CreateUnit(player owner, int typeId, float x, float y, float facing)
        {
            return Natives.CreateUnit(owner, typeId, x, y, facing);
        }

        /// <summary>Creates an item of a type at a place.</summary>
        public item CreateItem(int typeId, float x, float y)
        {
            return Natives.CreateItem(typeId, x, y);
        }

        /// <summary>The type id of a unit; 0 once the game has removed it.</summary>
        public int UnitTypeId(unit unit)
        {
            return Natives.GetUnitTypeId(unit);
        }

        /// <summary>The type id of an item; 0 once the game has removed it.</summary>
        public int ItemTypeId(item item)
        {
            return Natives.GetItemTypeId(item);
        }

        /// <summary>The type id of a destructable; 0 once the game has removed it.</summary>
        public int DestructableTypeId(destructable destructable)
        {
            return Natives.GetDestructableTypeId(destructable);
        }

        /// <summary>Removes a unit from the world.</summary>
        public void RemoveUnit(unit unit)
        {
            Natives.RemoveUnit(unit);
        }

        /// <summary>Removes an item from the world.</summary>
        public void RemoveItem(item item)
        {
            Natives.RemoveItem(item);
        }

        /// <summary>Removes a destructable from the world.</summary>
        public void RemoveDestructable(destructable destructable)
        {
            Natives.RemoveDestructable(destructable);
        }

        /// <summary>Whether a unit is a hero.</summary>
        public bool IsHero(unit unit)
        {
            return Natives.IsUnitType(unit, Natives.UNIT_TYPE_HERO);
        }

        /// <summary>The player that owns a unit.</summary>
        public player OwnerOf(unit unit)
        {
            return Natives.GetOwningPlayer(unit);
        }

        /// <summary>The level of an ability on a unit; 0 when the unit has none.</summary>
        public int AbilityLevel(unit unit, int abilityId)
        {
            return Natives.GetUnitAbilityLevel(unit, abilityId);
        }

        /// <summary>Gives a unit an ability.</summary>
        public void AddAbility(unit unit, int abilityId)
        {
            Natives.UnitAddAbility(unit, abilityId);
        }

        /// <summary>Hides an ability on a unit.</summary>
        public void HideAbility(unit unit, int abilityId)
        {
            Natives.BlzUnitHideAbility(unit, abilityId, true);
        }

        /// <summary>Takes an ability from a unit.</summary>
        public void RemoveAbility(unit unit, int abilityId)
        {
            Natives.UnitRemoveAbility(unit, abilityId);
        }

        /// <summary>Runs a callback once, after the map's main function has returned.</summary>
        internal static void Later(Action callback)
        {
            Natives.TimerStart(Natives.CreateTimer(), 0f, false, callback);
        }

        /// <summary>The clock's callback; the clock only measures time, so it does nothing.</summary>
        private static void ClockDone()
        {
        }

        /// <summary>Registers a player unit event for every player slot, with one trigger that runs the action.</summary>
        private static void Register(playerunitevent playerEvent, Action action)
        {
            var trigger = Natives.CreateTrigger();
            for (var slot = 0; slot < Natives.bj_MAX_PLAYER_SLOTS; slot++)
            {
                Natives.TriggerRegisterPlayerUnitEvent(trigger, Natives.Player(slot), playerEvent, null);
            }

            Natives.TriggerAddAction(trigger, action);
        }

        /// <summary>Hands a unit's entry to Inbound.</summary>
        private static void EnterAction()
        {
            try
            {
                Inbound.Entered(Natives.GetTriggerUnit());
            }
            catch (Exception error)
            {
                Errors.Report("Moonwell.Enter", error);
            }
        }

        /// <summary>Hands a unit's damage to Inbound, and sets the damage to the amount it returns.</summary>
        private static void DamageAction()
        {
            try
            {
                var amount = Natives.GetEventDamage();
                var result = Inbound.Damaged(
                    Natives.BlzGetEventDamageTarget(),
                    Natives.GetEventDamageSource(),
                    amount,
                    Natives.BlzGetEventIsAttack());
                if (result != amount)
                {
                    Natives.BlzSetEventDamage(result);
                }
            }
            catch (Exception error)
            {
                Errors.Report("Moonwell.Damage", error);
            }
        }

        /// <summary>Hands a unit's death to Inbound.</summary>
        private static void DeathAction()
        {
            try
            {
                Inbound.Died(Natives.GetDyingUnit(), Natives.GetKillingUnit());
            }
            catch (Exception error)
            {
                Errors.Report("Moonwell.Death", error);
            }
        }

        /// <summary>Hands a spell's effect to Inbound.</summary>
        private static void SpellEffectAction()
        {
            try
            {
                Inbound.SpellEffect(
                    Natives.GetSpellAbilityUnit(),
                    Natives.GetSpellAbilityId(),
                    Natives.GetSpellTargetUnit(),
                    Natives.GetSpellTargetX(),
                    Natives.GetSpellTargetY());
            }
            catch (Exception error)
            {
                Errors.Report("Moonwell.SpellEffect", error);
            }
        }

        /// <summary>Hands the end of a spell's cast to Inbound.</summary>
        private static void SpellEndedAction()
        {
            try
            {
                Inbound.SpellEnded(Natives.GetSpellAbilityUnit(), Natives.GetSpellAbilityId());
            }
            catch (Exception error)
            {
                Errors.Report("Moonwell.SpellEnded", error);
            }
        }

        /// <summary>Hands a pickup to Inbound.</summary>
        private static void PickUpAction()
        {
            try
            {
                Inbound.PickedUp(Natives.GetManipulatingUnit(), Natives.GetManipulatedItem());
            }
            catch (Exception error)
            {
                Errors.Report("Moonwell.PickUp", error);
            }
        }

        /// <summary>Hands a drop to Inbound.</summary>
        private static void DropAction()
        {
            try
            {
                Inbound.Dropped(Natives.GetManipulatingUnit(), Natives.GetManipulatedItem());
            }
            catch (Exception error)
            {
                Errors.Report("Moonwell.Drop", error);
            }
        }

        /// <summary>Hands a destructable's death to Inbound.</summary>
        private static void DestructableDeathAction()
        {
            try
            {
                Inbound.DestructableDied(Natives.GetTriggerDestructable());
            }
            catch (Exception error)
            {
                Errors.Report("Moonwell.DestructableDeath", error);
            }
        }

        /// <summary>Adds the item the enumeration is at to the items ItemsInWorld returns.</summary>
        private void CollectItem()
        {
            foundItems.Add(Natives.GetEnumItem());
        }

        /// <summary>Adds the destructable the enumeration is at to the destructables DestructablesInWorld returns.</summary>
        private void CollectDestructable()
        {
            foundDestructables.Add(Natives.GetEnumDestructable());
        }
    }
}
