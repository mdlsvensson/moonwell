namespace Moonwell.Testing
{
    /// <summary>A unit component on a dummy, which logs its damage and death, and on its first damage deals damage to Reply.</summary>
    public class DummyComp : UnitComponent
    {
        /// <summary>The unit that the first damage taken is answered with, once; null for none.</summary>
        public unit? Reply;

        /// <summary>Whether the answer has been dealt already.</summary>
        private bool replied;

        /// <summary>Logs the damage, then deals the answer on the first call only.</summary>
        public override void OnDamaged(Damage damage)
        {
            TestLog.Lines.Add("dummy comp damaged");
            if (Reply != null && !replied)
            {
                replied = true;
                Inbound.Damaged(Reply, Unit.Handle, 10f, false);
            }
        }

        /// <summary>Logs that the component saw the dummy die.</summary>
        public override void OnDeath(Unit? killer)
        {
            TestLog.Lines.Add("dummy comp died");
        }
    }
}
