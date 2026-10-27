using System;
using System.Collections.Generic;
using System.Reflection;

namespace Moonwell
{
    /// <summary>Finds which reactions a class overrides, once per class, and keeps the bit mask of each.</summary>
    internal static class Hooks
    {
        /// <summary>The mask of each class seen, keyed by the class.</summary>
        private static readonly Dictionary<object, int> masks = new Dictionary<object, int>();

        /// <summary>Set by a damage package: given the first object of each class whose mask is worked out; null without one.</summary>
        internal static Action<object>? Seen;

        /// <summary>Set by an interactions package: given each unit, item and destructable the runtime takes in, after its OnCreated; null without one.</summary>
        internal static Action<Widget>? TakenIn;

        /// <summary>Added by the packages that need it (interactions, areas): each is given every unit, item and destructable as it ends, before its OnLeave runs and while its place can still be read, so the interactions package has it out of its grid by its OnLeave (a gizmo, the package's own, leaves after). In the order added; empty without one.</summary>
        private static readonly List<Action<Widget>> endings = new List<Action<Widget>>();

        /// <summary>
        /// Whether obj's class, below declaringBase, defines name itself: for a base class, a method that overrides the
        /// base's reaction of that name, so a helper of the same name with other parameters, or a method that hides the
        /// reaction with new, does not count; for object (the packages' interface reactions), any public method of that
        /// name. In Lua MoonwellOverrides (moonwell.lua) walks the class chain, where CSharp.lua gives such a helper a name
        /// of its own; .NET asks reflection.
        /// </summary>
        /// @CSharpLua.Template = "MoonwellOverrides({0}, {1}, {2})"
        internal static bool Overrides(object obj, string name, Type declaringBase)
        {
            var methods = obj.GetType().GetMethods(BindingFlags.Public | BindingFlags.Instance);
            for (var i = 0; i < methods.Length; i++)
            {
                var method = methods[i];
                if (method.Name != name || method.DeclaringType == declaringBase)
                {
                    continue;
                }

                if (declaringBase == typeof(object) || method.GetBaseDefinition().DeclaringType == declaringBase)
                {
                    return true;
                }
            }

            return false;
        }

        /// <summary>The bit mask of the reactions obj's class overrides, worked out once per class.</summary>
        internal static int MaskOf(object obj, HookTable table)
        {
            var key = Names.ClassOf(obj);
            if (masks.TryGetValue(key, out var mask))
            {
                return mask;
            }

            mask = 0;
            for (var i = 0; i < table.Names.Length; i++)
            {
                if (Overrides(obj, table.Names[i], table.Base))
                {
                    mask |= table.Bits[i];
                }
            }

            masks[key] = mask;
            if (Seen != null)
            {
                Seen(obj);
            }

            return mask;
        }

        /// <summary>Tells the interactions package that a widget was taken in; an error in it is reported and goes no further.</summary>
        internal static void CallTakenIn(Widget widget)
        {
            if (TakenIn != null)
            {
                try
                {
                    TakenIn(widget);
                }
                catch (Exception error)
                {
                    Errors.Report("Moonwell.Hooks", error);
                }
            }
        }

        /// <summary>Adds a listener to the ending of every unit, item and destructable; it is called after those added before it. Runtime.Reset removes them all.</summary>
        internal static void AddEnding(Action<Widget> listener)
        {
            endings.Add(listener);
        }

        /// <summary>Tells each ending listener, in the order added, that a widget is ending; an error in one is reported and the next still runs.</summary>
        internal static void CallEnding(Widget widget)
        {
            for (var i = 0; i < endings.Count; i++)
            {
                try
                {
                    endings[i](widget);
                }
                catch (Exception error)
                {
                    Errors.Report("Moonwell.Hooks", error);
                }
            }
        }

        /// <summary>Forgets every class's mask and the hooks the packages set; Runtime.Reset calls it.</summary>
        internal static void Reset()
        {
            masks.Clear();
            Seen = null;
            TakenIn = null;
            endings.Clear();
        }
    }
}
