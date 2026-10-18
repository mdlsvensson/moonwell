using System;

namespace Moonwell.Objects
{
    /// <summary>The values of a field that has one for each level, counted from level 1.</summary>
    /// @CSharpLua.Ignore
    public sealed class Levels<T>
    {
        private readonly Func<int, T>? formula;
        private T[] values;
        private bool counted;

        /// <summary>Holds one value for each level, in order.</summary>
        internal Levels(T[] values)
        {
            this.values = values;
            counted = true;
        }

        /// <summary>Holds the value of each level as a formula of the level; the count is unknown until <see cref="Resolve"/>.</summary>
        internal Levels(Func<int, T> formula)
        {
            this.formula = formula;
            values = Array.Empty<T>();
        }

        /// <summary>The number of levels; 0 for a formula that has not been resolved.</summary>
        public int Count
        {
            get { return values.Length; }
        }

        /// <summary>The value of a level, counted from 1; a formula that has not been resolved answers any level from 1.</summary>
        public T this[int level]
        {
            get
            {
                if (level < 1 || (counted && level > values.Length))
                {
                    throw new ArgumentOutOfRangeException(nameof(level), level, "Levels count from 1 to " + values.Length);
                }

                return counted ? values[level - 1] : formula!(level);
            }
        }

        /// <summary>The value of each level, in order; empty for a formula that has not been resolved.</summary>
        internal T[] Values
        {
            get { return values; }
        }

        /// <summary>Fixes the number of levels of a formula and works out the value of each; does nothing for values given one by one.</summary>
        internal void Resolve(int count)
        {
            if (formula == null)
            {
                return;
            }

            var resolved = new T[count];
            for (var i = 0; i < count; i++)
            {
                resolved[i] = formula(i + 1);
            }

            values = resolved;
            counted = true;
        }
    }
}
