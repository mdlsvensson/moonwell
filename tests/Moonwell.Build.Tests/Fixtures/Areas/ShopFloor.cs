using Moonwell;

namespace AreasFixture
{
    /// <summary>The map's "Shop" region, where a unit that comes in is greeted and a unit inside is healed each second.</summary>
    public sealed class ShopFloor : Area
    {
        public ShopFloor()
        {
            Shape = AreaShape.Region("Shop");
            Tick = 1f;
        }

        public override void OnUnitEntered(Unit enteringUnit)
        {
            enteringUnit.Owner.Print("Welcome.");
        }

        public override void OnUnitStays(Unit stayingUnit)
        {
            stayingUnit.Life += 20f;
        }
    }
}
