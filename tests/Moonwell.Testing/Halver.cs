namespace Moonwell.Testing
{
    /// <summary>A unit component that halves the damage its unit takes.</summary>
    public class Halver : UnitComponent
    {
        /// <summary>Halves the amount of the damage.</summary>
        public override void OnDamaged(Damage damage)
        {
            damage.Amount = damage.Amount / 2f;
        }
    }
}
