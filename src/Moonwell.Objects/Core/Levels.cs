using System;

namespace Moonwell.Objects
{
    /// <summary>The values of a field that has one for each level, counted from level 1.</summary>
    /// @CSharpLua.Ignore
    public sealed class Levels<T> : ILevelValues
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

    /// <summary>The levels of a value as the rule checks read them, whatever the type of a level.</summary>
    /// @CSharpLua.Ignore
    internal interface ILevelValues
    {
        /// <summary>True when the number of levels is known: values given one by one, or a formula resolved.</summary>
        bool Counted { get; }

        /// <summary>The number of levels; 0 for a formula not yet resolved.</summary>
        int Count { get; }

        /// <summary>The value of a level, counted from 0.</summary>
        object? ValueAt(int index);

        /// <summary>Calls <paramref name="check"/> with each level (counted from 1) and its value when a formula is resolved.</summary>
        void WhenCounted(Action<int, object?> check);
    }
}
