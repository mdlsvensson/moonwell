using System;
using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>Runs a stage's reactions on everything a hit involves, in decision 12's order: the source and its components, the target and its components, the game's components, the target's player's, then the source's player's.</summary>
    internal static class Reactions
    {
        /// <summary>Where each interface's methods start among the slots: a method's slot is this plus its stage.</summary>
        private const int SourceSlots = 0, TargetSlots = 5, WatchSlots = 10;

        /// <summary>The reaction methods by slot, five to an interface in stage order (before armor, after armor, lethal, landed, burst); the target has no burst. A slot's bit is 1 shifted by the slot, and a class's mask holds the bits of the methods it writes.</summary>
        private static readonly string?[] names =
        {
            "OnDealing", "OnDealt", "OnKilling", "OnLanded", "OnBurst",
            "OnTaking", "OnTaken", "OnDying", "OnHurt", null,
            "OnUnitTaking", "OnUnitTaken", "OnUnitDying", "OnUnitHurt", "OnUnitBurst",
        };

        /// <summary>The mask of each class seen, keyed by the class.</summary>
        private static readonly Dictionary<object, int> masks = new Dictionary<object, int>();

        /// <summary>The bits of every class seen, together: a stage none of them reacts to is skipped.</summary>
        private static int seen;

        /// <summary>The hits whose reactions are running, innermost last; a hit made while they run belongs to the chain of the last.</summary>
        internal static readonly List<Hit> Running = new List<Hit>();

        /// <summary>Forgets every class's mask and the running hits; Install calls it.</summary>
        internal static void Reset()
        {
            masks.Clear();
            seen = 0;
            Running.Clear();
        }

        /// <summary>Works out the reactions of a class the runtime sees for the first time, so the stages it reacts to are run.</summary>
        internal static void Learn(object obj)
        {
            MaskOf(obj);
        }

        /// <summary>Whether any class seen reacts to a stage.</summary>
        internal static bool Wants(Stage stage)
        {
            var bits = (1 << (SourceSlots + (int)stage)) | (1 << (TargetSlots + (int)stage)) | (1 << (WatchSlots + (int)stage));
            return (seen & bits) != 0;
        }

        /// <summary>The chain place of a hit made now: 0, or one more than the hit whose reactions are running.</summary>
        internal static int NextChain()
        {
            return Running.Count == 0 ? 0 : Running[Running.Count - 1].Chain + 1;
        }

        /// <summary>Runs core's OnDamaged family on a hit, as reactions to it, so damage they deal continues its chain.</summary>
        internal static void RunCore(Hit hit)
        {
            Running.Add(hit);
            try
            {
                Inbound.Dispatch(hit);
            }
            finally
            {
                Running.RemoveAt(Running.Count - 1);
            }
        }

        /// <summary>Runs a stage's reactions to a hit; a quiet hit has none.</summary>
        internal static void Run(Hit hit, Stage stage)
        {
            if (hit.Quiet || !Wants(stage))
            {
                return;
            }

            var source = hit.Source;
            var target = hit.Target;
            var targetSlot = target.Player.Slot;
            var sourceSlot = source == null ? -1 : source.Player.Slot;
            if (source != null)
            {
                RunUnit(source, hit, null, SourceSlots + (int)stage);
            }

            RunUnit(target, hit, null, TargetSlots + (int)stage);
            RunMap(Game.Instance, hit, null, WatchSlots + (int)stage);
            var first = Player.Existing(targetSlot);
            if (first != null)
            {
                RunMap(first, hit, null, WatchSlots + (int)stage);
            }

            if (source != null && sourceSlot != targetSlot)
            {
                var second = Player.Existing(sourceSlot);
                if (second != null)
                {
                    RunMap(second, hit, null, WatchSlots + (int)stage);
                }
            }
        }

        /// <summary>Runs the burst reactions: the source and its components, the game's components, then the source's player's.</summary>
        internal static void RunBurst(Burst burst)
        {
            var slot = burst.Source.Player.Slot;
            RunUnit(burst.Source, null, burst, SourceSlots + (int)Stage.Burst);
            RunMap(Game.Instance, null, burst, WatchSlots + (int)Stage.Burst);
            var player = Player.Existing(slot);
            if (player != null)
            {
                RunMap(player, null, burst, WatchSlots + (int)Stage.Burst);
            }
        }

        /// <summary>Runs a unit's reaction in a slot, then its components', in add order; a unit or component that has ended is skipped.</summary>
        private static void RunUnit(Unit unit, Hit? hit, Burst? burst, int slot)
        {
            var bit = 1 << slot;
            if (!unit.HasEnded && (MaskOf(unit) & bit) != 0 && !Spared(hit))
            {
                Call(unit, hit, burst, slot);
            }

            var count = unit.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = unit.ComponentAt(i) as UnitComponent;
                if (component != null && !component.HasEnded && (MaskOf(component) & bit) != 0 && !Spared(hit))
                {
                    Call(component, hit, burst, slot);
                }
            }

            unit.EndDispatch();
        }

        /// <summary>Runs the map-wide reaction in a slot on an actor's components, in add order.</summary>
        private static void RunMap(Actor actor, Hit? hit, Burst? burst, int slot)
        {
            var bit = 1 << slot;
            var count = actor.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = actor.ComponentAt(i) as MapComponent;
                if (component != null && !component.HasEnded && (MaskOf(component) & bit) != 0 && !Spared(hit))
                {
                    Call(component, hit, burst, slot);
                }
            }

            actor.EndDispatch();
        }

        /// <summary>Whether a hit in its lethal stage no longer kills, as an earlier reaction lowered it: the later lethal reactions do not run.</summary>
        private static bool Spared(Hit? hit)
        {
            return hit != null && hit.Stage == Stage.Lethal && !hit.Kills;
        }

        /// <summary>Calls the reaction in a slot with the object as the current owner; a failing reaction is reported.</summary>
        private static void Call(Scope obj, Hit? hit, Burst? burst, int slot)
        {
            var previous = Scope.current;
            Scope.current = obj;
            var running = burst != null ? burst.Hits[0] : hit!;
            Running.Add(running);
            try
            {
                switch (slot)
                {
                    case SourceSlots + (int)Stage.BeforeArmor: ((IDamageSource)obj).OnDealing(hit!); break;
                    case SourceSlots + (int)Stage.AfterArmor: ((IDamageSource)obj).OnDealt(hit!); break;
                    case SourceSlots + (int)Stage.Lethal: ((IDamageSource)obj).OnKilling(hit!); break;
                    case SourceSlots + (int)Stage.Landed: ((IDamageSource)obj).OnLanded(hit!); break;
                    case SourceSlots + (int)Stage.Burst: ((IDamageSource)obj).OnBurst(burst!); break;
                    case TargetSlots + (int)Stage.BeforeArmor: ((IDamageTarget)obj).OnTaking(hit!); break;
                    case TargetSlots + (int)Stage.AfterArmor: ((IDamageTarget)obj).OnTaken(hit!); break;
                    case TargetSlots + (int)Stage.Lethal: ((IDamageTarget)obj).OnDying(hit!); break;
                    case TargetSlots + (int)Stage.Landed: ((IDamageTarget)obj).OnHurt(hit!); break;
                    case WatchSlots + (int)Stage.BeforeArmor: ((IDamageWatch)obj).OnUnitTaking(hit!); break;
                    case WatchSlots + (int)Stage.AfterArmor: ((IDamageWatch)obj).OnUnitTaken(hit!); break;
                    case WatchSlots + (int)Stage.Lethal: ((IDamageWatch)obj).OnUnitDying(hit!); break;
                    case WatchSlots + (int)Stage.Landed: ((IDamageWatch)obj).OnUnitHurt(hit!); break;
                    case WatchSlots + (int)Stage.Burst: ((IDamageWatch)obj).OnUnitBurst(burst!); break;
                }
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(obj) + "." + names[slot], error);
            }

            Running.RemoveAt(Running.Count - 1);
            Scope.current = previous;
        }

        /// <summary>The bits of the reaction methods an object's class writes, worked out once per class. Only methods the class writes count: in Lua an interface's empty defaults are not there.</summary>
        private static int MaskOf(object obj)
        {
            var key = Names.ClassOf(obj);
            if (masks.TryGetValue(key, out var mask))
            {
                return mask;
            }

            mask = 0;
            for (var slot = 0; slot < names.Length; slot++)
            {
                var name = names[slot];
                if (name != null && Implements(obj, slot) && Hooks.Overrides(obj, name, typeof(object)))
                {
                    mask |= 1 << slot;
                }
            }

            masks[key] = mask;
            seen |= mask;
            return mask;
        }

        /// <summary>Whether an object implements the interface a slot belongs to.</summary>
        private static bool Implements(object obj, int slot)
        {
            if (slot < TargetSlots)
            {
                return obj is IDamageSource;
            }

            return slot < WatchSlots ? obj is IDamageTarget : obj is IDamageWatch;
        }
    }
}
