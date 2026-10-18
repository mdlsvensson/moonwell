namespace Moonwell.Objects.Tests
{
    /// <summary>A kind base written by hand, in the pattern the generated kind bases follow.</summary>
    public class TestUnitBase : Moonwell.Unit
    {
        private Declared? declared;

        /// <summary>The declared values of this object, created on first use.</summary>
        protected Declared Fields => declared ??= new Declared();

        /// <summary>The hit points.</summary>
        [Field("uhpm")]
        public int HitPoints
        {
            get { return Fields.Get<int>(this, "uhpm"); }
            protected set { Fields.Put("uhpm", value); }
        }

        /// <summary>The damage of each attack level.</summary>
        [Field("uatk")]
        public Levels<float> Damage
        {
            get { return Fields.GetLevels<float>(this, "uatk"); }
            protected set { Fields.Put("uatk", value); }
        }

        /// <summary>The units this one can train.</summary>
        [Field("utra")]
        public ObjectList<Moonwell.Unit> Trains
        {
            get { return Fields.GetList<Moonwell.Unit>("utra"); }
        }

        /// <summary>The unit this one upgrades to.</summary>
        [Field("uupt")]
        public ObjectRef<Moonwell.Unit> UpgradesTo
        {
            get { return Fields.GetRef<Moonwell.Unit>("uupt"); }
        }

        /// <summary>The values of the declared class, as put so far.</summary>
        public IReadOnlyDictionary<string, object?> Declared => Fields.Values;

        /// <summary>Makes a list of per-level values.</summary>
        protected static Levels<T> Levels<T>(params T[] values)
        {
            return new Levels<T>(values);
        }

        /// <summary>Makes per-level values from a formula of the level.</summary>
        protected static Levels<T> PerLevel<T>(Func<int, T> formula)
        {
            return new Levels<T>(formula);
        }

        /// <summary>Puts a value by its field id.</summary>
        protected void Set(string field, object value)
        {
            Fields.Put(field, value);
        }

        /// <summary>Puts the value of one level by its field id.</summary>
        protected void Set(string field, int level, object value)
        {
            Fields.Put(field + ":" + level, value);
        }
    }
}
