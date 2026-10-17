namespace Moonwell.Testing
{
    /// <summary>A unit that starts with a DummyComp, logs when it is damaged and when it dies, and keeps the killer it was given.</summary>
    public class Dummy : Unit
    {
        /// <summary>The killer the last OnDeath was given; null when none.</summary>
        public Unit? LastKiller;

        /// <summary>Makes a dummy that starts with a DummyComp.</summary>
        public Dummy()
        {
            Components.Add(new DummyComp());
        }

        /// <summary>Logs that the dummy took damage.</summary>
        public override void OnDamaged(Damage damage)
        {
            TestLog.Lines.Add("dummy damaged");
        }

        /// <summary>Keeps the killer, and logs that the dummy died.</summary>
        public override void OnDeath(Unit? killer)
        {
            LastKiller = killer;
            TestLog.Lines.Add("dummy died");
        }
    }
}
