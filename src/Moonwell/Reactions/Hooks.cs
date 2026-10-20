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

        // Whether obj's class, below declaringBase, defines name itself. In Lua MoonwellOverrides (moonwell.lua) walks
        // the class chain; .NET asks reflection. No summary goes above this line: CSharp.lua then ignores the template.
        /// @CSharpLua.Template = "MoonwellOverrides({0}, {1}, {2})"
        internal static bool Overrides(object obj, string name, Type declaringBase)
        {
            var method = obj.GetType().GetMethod(name, BindingFlags.Public | BindingFlags.Instance);
            return method != null && method.DeclaringType != declaringBase;
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

        /// <summary>Forgets every class's mask; Runtime.Reset calls it.</summary>
        internal static void Reset()
        {
            masks.Clear();
            Seen = null;
        }
    }
}
