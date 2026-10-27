using System;
using System.Collections.Generic;

namespace Moonwell.Objects
{
    /// <summary>The values of a field that has one for each level, counted from level 1.</summary>
    /// @CSharpLua.Ignore
    public sealed class Levels<T>
#if !MOONWELL_LUA
        : ILevelValues
#endif
    {
        private readonly Func<int, T>? formula;
        private T[] values;
        private bool counted;
        private Action<int, object?>? whenCounted;

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

        /// <summary>One value for every level: <c>Name = "Sharpened Blades"</c>, as <c>PerLevel(level => "Sharpened Blades")</c>.</summary>
        public static implicit operator Levels<T>(T value)
        {
            return new Levels<T>(level => value);
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

#if !MOONWELL_LUA
        bool ILevelValues.Counted
        {
            get { return counted; }
        }

        object? ILevelValues.ValueAt(int index)
        {
            return values[index];
        }

        void ILevelValues.WhenCounted(Action<int, object?> check)
        {
            whenCounted += check;
        }
#endif

        /// <summary>
        /// These values with some levels put by themselves (<c>Set("Htb1", 2, 250f)</c>), as the build writes them: a copy of
        /// the values given one by one with the puts at their levels (a put past them is left out), or a formula that gives
        /// a put level its put value.
        /// </summary>
        internal Levels<T> WithLevels(IReadOnlyDictionary<int, T> puts)
        {
            if (counted)
            {
                var copy = (T[])values.Clone();
                foreach (var put in puts)
                {
                    if (put.Key >= 1 && put.Key <= copy.Length)
                    {
                        copy[put.Key - 1] = put.Value;
                    }
                }

                return new Levels<T>(copy);
            }

            var formula = this.formula!;
            return new Levels<T>(level => puts.TryGetValue(level, out var one) ? one : formula(level));
        }

        /// <summary>
        /// Fixes the number of levels of a formula and works out the value of each, checking each against the rules of the
        /// fields it was put on (level 1 first); does nothing for values given one by one.
        /// </summary>
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
                whenCounted?.Invoke(i + 1, resolved[i]);
            }

            values = resolved;
            counted = true;
        }
    }
}
