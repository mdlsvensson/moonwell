using System;

namespace Moonwell
{
    /// <summary>The reactions a base class declares: each name and its bit, and the base that declares them.</summary>
    internal sealed class HookTable
    {
        /// <summary>The base class whose reactions are listed; a method declared here is not an override.</summary>
        internal readonly Type Base;

        /// <summary>The method name of each reaction.</summary>
        internal readonly string[] Names;

        /// <summary>The bit of each reaction, in the same order as Names.</summary>
        internal readonly int[] Bits;

        /// <summary>Makes a table for a base class's reactions.</summary>
        internal HookTable(Type declaringBase, string[] names, int[] bits)
        {
            Base = declaringBase;
            Names = names;
            Bits = bits;
        }
    }
}
