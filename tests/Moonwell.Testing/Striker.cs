namespace Moonwell.Testing
{
    /// <summary>A unit that starts with a StrikerComp, and logs when it deals damage and when it kills.</summary>
    public class Striker : Unit
    {
        /// <summary>Makes a striker that starts with a StrikerComp.</summary>
        public Striker()
        {
            Components.Add(new StrikerComp());
        }

        /// <summary>Logs that the striker dealt damage.</summary>
        public override void OnDealtDamage(Damage damage)
        {
            TestLog.Lines.Add("striker dealt");
        }

        /// <summary>Logs that the striker killed a unit.</summary>
        public override void OnKill(Unit victim)
        {
            TestLog.Lines.Add("striker killed");
        }
    }
}
