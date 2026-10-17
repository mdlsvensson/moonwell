namespace Moonwell.Testing
{
    /// <summary>A unit component on a striker, which logs when the striker deals damage and when it kills.</summary>
    public class StrikerComp : UnitComponent
    {
        /// <summary>Logs that the component saw the striker deal damage.</summary>
        public override void OnDealtDamage(Damage damage)
        {
            TestLog.Lines.Add("striker comp dealt");
        }

        /// <summary>Logs that the component saw the striker kill.</summary>
        public override void OnKill(Unit victim)
        {
            TestLog.Lines.Add("striker comp killed");
        }
    }
}
