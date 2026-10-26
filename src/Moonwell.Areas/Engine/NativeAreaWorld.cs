using System;
using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell.Engine
{
    /// <summary>The game as the areas package sees it, through the game's natives; the only class of the package that calls them.</summary>
    internal sealed class NativeAreaWorld : IAreaWorld
    {
        /// <summary>The two triggers of a region.</summary>
        private sealed class Triggers
        {
            internal trigger? Entered;

            internal trigger? Left;
        }

        /// <summary>The triggers of each region Listen has been given.</summary>
        private readonly Dictionary<region, Triggers> listened = new Dictionary<region, Triggers>();

        /// <summary>The unit of the event being delivered; saved and put back around each, as an event can start another.</summary>
        private unit? current;

        /// <summary>The group UnitsInRect fills, made on first use.</summary>
        private group? scratch;

        /// <summary>Makes an empty region.</summary>
        public region CreateRegion() => Natives.CreateRegion();

        /// <summary>Adds a rect to a region: the game rounds it out to whole cells.</summary>
        public void AddRect(region r, float minX, float minY, float maxX, float maxY)
        {
            var made = Natives.Rect(minX, minY, maxX, maxY);
            Natives.RegionAddRect(r, made);
            Natives.RemoveRect(made);
        }

        /// <summary>Adds the cell that holds a point to a region.</summary>
        public void AddCell(region r, float x, float y) => Natives.RegionAddCell(r, x, y);

        /// <summary>Makes the enter and leave triggers of a region.</summary>
        public void Listen(region r, Action entered, Action left)
        {
            var triggers = new Triggers();
            var enter = Natives.CreateTrigger();
            Natives.TriggerRegisterEnterRegion(enter, r, null);
            Natives.TriggerAddAction(enter, () => Deliver(Natives.GetEnteringUnit(), entered));
            var leave = Natives.CreateTrigger();
            Natives.TriggerRegisterLeaveRegion(leave, r, null);
            Natives.TriggerAddAction(leave, () => Deliver(Natives.GetLeavingUnit(), left));
            triggers.Entered = enter;
            triggers.Left = leave;
            listened[r] = triggers;
        }

        /// <summary>The unit of the event being delivered.</summary>
        public unit EventUnit() => current!;

        /// <summary>Destroys a region and the two triggers made for it.</summary>
        public void DestroyRegion(region r)
        {
            if (listened.TryGetValue(r, out var triggers))
            {
                Natives.DestroyTrigger(triggers.Entered!);
                Natives.DestroyTrigger(triggers.Left!);
                listened.Remove(r);
            }

            Natives.RemoveRegion(r);
        }

        /// <summary>Whether a unit is inside a region.</summary>
        public bool IsInside(region r, unit u) => Natives.IsUnitInRegion(r, u);

        /// <summary>Adds the units whose centre is in a rect to a list.</summary>
        public void UnitsInRect(float minX, float minY, float maxX, float maxY, List<unit> into)
        {
            if (scratch == null)
            {
                scratch = Natives.CreateGroup();
            }

            var made = Natives.Rect(minX, minY, maxX, maxY);
            Natives.GroupEnumUnitsInRect(scratch, made, null);
            Natives.RemoveRect(made);
            var count = Natives.BlzGroupGetSize(scratch);
            for (var i = 0; i < count; i++)
            {
                into.Add(Natives.BlzGroupUnitAt(scratch, i));
            }

            Natives.GroupClear(scratch);
        }

        /// <summary>Registers the death and the hero revive events for every player slot, with one trigger each.</summary>
        public void ListenDeaths(Action died, Action revived)
        {
            var death = Natives.CreateTrigger();
            var revive = Natives.CreateTrigger();
            for (var slot = 0; slot < Natives.bj_MAX_PLAYER_SLOTS; slot++)
            {
                Natives.TriggerRegisterPlayerUnitEvent(death, Natives.Player(slot), Natives.EVENT_PLAYER_UNIT_DEATH, null);
                Natives.TriggerRegisterPlayerUnitEvent(revive, Natives.Player(slot), Natives.EVENT_PLAYER_HERO_REVIVE_FINISH, null);
            }

            Natives.TriggerAddAction(death, () => Deliver(Natives.GetDyingUnit(), died));
            Natives.TriggerAddAction(revive, () => Deliver(Natives.GetRevivingUnit(), revived));
        }

        /// <summary>Whether a unit is alive, not hidden and not loaded into a transport.</summary>
        public bool HasPlace(unit u) => Natives.UnitAlive(u) && !Natives.IsUnitHidden(u) && !Natives.IsUnitLoaded(u);

        /// <summary>Runs an event's action with the unit set, and puts the previous unit back after; a failing action is reported.</summary>
        private void Deliver(unit target, Action action)
        {
            var before = current;
            current = target;
            try
            {
                action();
            }
            catch (Exception error)
            {
                Errors.Report("Moonwell.Areas", error);
            }
            finally
            {
                current = before;
            }
        }
    }
}
