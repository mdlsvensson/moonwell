using System;
using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>Runs a stage's reactions on everything a hit involves, in decision 12's order: the source and its components, the target and its components, the game's components, the target's player's, then the source's player's.</summary>
    internal static class Reactions
    {
        /// <summary>The reaction methods, one bit each: a class's mask holds the ones it has.</summary>
        private const int Dealing = 1, Dealt = 2, Killing = 4, Landed = 8, Burst = 16,
            Taking = 32, Taken = 64, Lethal = 128, Hurt = 256,
            BeforeArmor = 512, AfterArmor = 1024, WatchLethal = 2048, WatchLanded = 4096, WatchBurst = 8192;

        /// <summary>The mask of each class seen, keyed by the class.</summary>
        private static readonly Dictionary<object, int> masks = new Dictionary<object, int>();

        /// <summary>The hits whose reactions are running, innermost last; a hit dealt from a reaction belongs to the chain of the last.</summary>
        internal static readonly List<Hit> Running = new List<Hit>();

        /// <summary>Forgets every class's mask and the running hits; Install calls it.</summary>
        internal static void Reset()
        {
            masks.Clear();
            Running.Clear();
        }

        /// <summary>Runs a stage's reactions to a hit.</summary>
        internal static void Run(Hit hit, Stage stage)
        {
            var source = hit.Source;
            var target = hit.Target;
            var targetSlot = target.Player.Slot;
            var sourceSlot = source == null ? -1 : source.Player.Slot;
            if (source != null)
            {
                RunUnit(source, hit, null, stage, true);
            }

            RunUnit(target, hit, null, stage, false);
            RunMap(Game.Instance, hit, null, stage);
            var first = Player.Existing(targetSlot);
            if (first != null)
            {
                RunMap(first, hit, null, stage);
            }

            if (source != null && sourceSlot != targetSlot)
            {
                var second = Player.Existing(sourceSlot);
                if (second != null)
                {
                    RunMap(second, hit, null, stage);
                }
            }
        }

        /// <summary>Runs the burst reactions: the source and its components, the game's components, then the source's player's.</summary>
        internal static void RunBurst(Burst burst)
        {
            var slot = burst.Source.Player.Slot;
            RunUnit(burst.Source, null, burst, Stage.Landed, true);
            RunMap(Game.Instance, null, burst, Stage.Landed);
            var player = Player.Existing(slot);
            if (player != null)
            {
                RunMap(player, null, burst, Stage.Landed);
            }
        }

        /// <summary>Runs a unit's reactions, then its components', in add order; a unit or component that has ended is skipped.</summary>
        private static void RunUnit(Unit unit, Hit? hit, Burst? burst, Stage stage, bool isSource)
        {
            var bit = burst != null ? Burst : isSource ? SourceBit(stage) : TargetBit(stage);
            if (!unit.HasEnded && (MaskOf(unit) & bit) != 0 && !Spared(hit, stage))
            {
                Call(unit, hit, burst, bit);
            }

            var count = unit.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = unit.ComponentAt(i) as UnitComponent;
                if (component != null && !component.HasEnded && (MaskOf(component) & bit) != 0 && !Spared(hit, stage))
                {
                    Call(component, hit, burst, bit);
                }
            }

            unit.EndDispatch();
        }

        /// <summary>Runs the map-wide reactions on an actor's components, in add order.</summary>
        private static void RunMap(Actor actor, Hit? hit, Burst? burst, Stage stage)
        {
            var bit = burst != null ? WatchBurst : WatchBit(stage);
            var count = actor.BeginDispatch();
            for (var i = 0; i < count; i++)
            {
                var component = actor.ComponentAt(i) as MapComponent;
                if (component != null && !component.HasEnded && (MaskOf(component) & bit) != 0 && !Spared(hit, stage))
                {
                    Call(component, hit, burst, bit);
                }
            }

            actor.EndDispatch();
        }

        /// <summary>Whether a lethal stage's hit no longer kills, as an earlier reaction lowered it: the later lethal reactions do not run.</summary>
        private static bool Spared(Hit? hit, Stage stage)
        {
            return stage == Stage.Lethal && hit != null && !hit.Kills;
        }

        /// <summary>Calls one reaction with the object as the current owner; a failing reaction is reported.</summary>
        private static void Call(Owner obj, Hit? hit, Burst? burst, int bit)
        {
            var previous = Owner.current;
            Owner.current = obj;
            var running = burst != null ? burst.Hits[0] : hit!;
            Running.Add(running);
            try
            {
                switch (bit)
                {
                    case Dealing: ((IDamageSource)obj).OnDealing(hit!); break;
                    case Dealt: ((IDamageSource)obj).OnDealt(hit!); break;
                    case Killing: ((IDamageSource)obj).OnKilling(hit!); break;
                    case Landed: ((IDamageSource)obj).OnLanded(hit!); break;
                    case Burst: ((IDamageSource)obj).OnBurst(burst!); break;
                    case Taking: ((IDamageTarget)obj).OnTaking(hit!); break;
                    case Taken: ((IDamageTarget)obj).OnTaken(hit!); break;
                    case Lethal: ((IDamageTarget)obj).OnLethal(hit!); break;
                    case Hurt: ((IDamageTarget)obj).OnHurt(hit!); break;
                    case BeforeArmor: ((IDamageWatch)obj).OnBeforeArmor(hit!); break;
                    case AfterArmor: ((IDamageWatch)obj).OnAfterArmor(hit!); break;
                    case WatchLethal: ((IDamageWatch)obj).OnLethal(hit!); break;
                    case WatchLanded: ((IDamageWatch)obj).OnLanded(hit!); break;
                    case WatchBurst: ((IDamageWatch)obj).OnBurst(burst!); break;
                }
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(obj) + "." + MethodName(bit), error);
            }

            Running.RemoveAt(Running.Count - 1);
            Owner.current = previous;
        }

        /// <summary>The source's method of a stage.</summary>
        private static int SourceBit(Stage stage) => stage == Stage.BeforeArmor ? Dealing : stage == Stage.AfterArmor ? Dealt : stage == Stage.Lethal ? Killing : Landed;

        /// <summary>The target's method of a stage.</summary>
        private static int TargetBit(Stage stage) => stage == Stage.BeforeArmor ? Taking : stage == Stage.AfterArmor ? Taken : stage == Stage.Lethal ? Lethal : Hurt;

        /// <summary>The map-wide method of a stage.</summary>
        private static int WatchBit(Stage stage) => stage == Stage.BeforeArmor ? BeforeArmor : stage == Stage.AfterArmor ? AfterArmor : stage == Stage.Lethal ? WatchLethal : WatchLanded;

        /// <summary>The bits of the reaction methods an object's class has, worked out once per class. Only methods the class writes count: in Lua an interface's empty defaults are not there.</summary>
        private static int MaskOf(object obj)
        {
            var key = Names.ClassOf(obj);
            if (masks.TryGetValue(key, out var mask))
            {
                return mask;
            }

            mask = 0;
            if (obj is IDamageSource)
            {
                mask |= Has(obj, "OnDealing", Dealing) | Has(obj, "OnDealt", Dealt) | Has(obj, "OnKilling", Killing) | Has(obj, "OnLanded", Landed) | Has(obj, "OnBurst", Burst);
            }

            if (obj is IDamageTarget)
            {
                mask |= Has(obj, "OnTaking", Taking) | Has(obj, "OnTaken", Taken) | Has(obj, "OnLethal", Lethal) | Has(obj, "OnHurt", Hurt);
            }

            if (obj is IDamageWatch)
            {
                mask |= Has(obj, "OnBeforeArmor", BeforeArmor) | Has(obj, "OnAfterArmor", AfterArmor) | Has(obj, "OnLethal", WatchLethal) | Has(obj, "OnLanded", WatchLanded) | Has(obj, "OnBurst", WatchBurst);
            }

            masks[key] = mask;
            return mask;
        }

        /// <summary>The bit when the object's class writes the method, else 0.</summary>
        private static int Has(object obj, string name, int bit)
        {
            return Hooks.Overrides(obj, name, typeof(object)) ? bit : 0;
        }

        /// <summary>The method name of a bit, for an error line.</summary>
        private static string MethodName(int bit)
        {
            switch (bit)
            {
                case Dealing: return "OnDealing";
                case Dealt: return "OnDealt";
                case Killing: return "OnKilling";
                case Landed: return "OnLanded";
                case Burst: return "OnBurst";
                case Taking: return "OnTaking";
                case Taken: return "OnTaken";
                case Lethal: return "OnLethal";
                case Hurt: return "OnHurt";
                case BeforeArmor: return "OnBeforeArmor";
                case AfterArmor: return "OnAfterArmor";
                case WatchLethal: return "OnLethal";
                case WatchLanded: return "OnLanded";
                default: return "OnBurst";
            }
        }
    }
}
