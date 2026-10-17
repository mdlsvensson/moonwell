namespace Moonwell.Testing
{
    /// <summary>A unit component that logs when its unit takes damage.</summary>
    public class B : UnitComponent
    {
        /// <summary>Logs that the damage reached the component.</summary>
        public override void OnDamaged(Damage damage)
        {
            TestLog.Lines.Add("b damaged");
        }
    }
}
