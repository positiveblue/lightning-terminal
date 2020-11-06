import React from 'react';
import { observer } from 'mobx-react-lite';
import { usePrefixedTranslation } from 'hooks';
import { Unit, Units } from 'util/constants';
import { useStore } from 'store';
import { Button, Section, Small, SummaryItem } from 'components/base';
import FormField from 'components/common/FormField';
import FormInputNumber from 'components/common/FormInputNumber';
import Toggle from 'components/common/Toggle';
import { styled } from 'components/theme';

const Styled = {
  Section: styled(Section)`
    flex: 1;
  `,
  ApySummaryItem: styled(SummaryItem)`
    margin-top: 50px;
  `,
  Small: styled(Small)`
    color: ${props => props.theme.colors.gray};
  `,
  Actions: styled.div`
    margin: 30px auto;
    text-align: center;
  `,
};

const OrderFormSection: React.FC = () => {
  const { l } = usePrefixedTranslation('cmps.pool.OrderFormSection');
  const { orderFormStore } = useStore();

  const { Section, ApySummaryItem, Small, Actions } = Styled;
  return (
    <Section>
      <Actions>
        <Toggle
          flex
          options={orderFormStore.orderOptions}
          value={orderFormStore.orderType}
          onChange={orderFormStore.setOrderType}
        />
      </Actions>
      <FormField
        label={l(`amountLabel${orderFormStore.orderType}`)}
        error={orderFormStore.amountError}
      >
        <FormInputNumber
          placeholder={l('amountPlaceholder')}
          extra={Units[Unit.sats].suffix}
          value={orderFormStore.amount}
          onChange={orderFormStore.setAmount}
        />
      </FormField>
      <FormField
        label={l(`premiumLabel${orderFormStore.orderType}`)}
        error={orderFormStore.premiumError}
      >
        <FormInputNumber
          placeholder={l('premiumPlaceholder')}
          value={orderFormStore.premium}
          onChange={orderFormStore.setPremium}
          extra={
            <>
              <Button
                ghost
                borderless
                compact
                onClick={orderFormStore.setSuggestedPremium}
              >
                {l('premiumSuggested')}
              </Button>
              <span>{Units[Unit.sats].suffix}</span>
            </>
          }
        />
      </FormField>
      <FormField label={l('minChanSizeLabel')} error={orderFormStore.minChanSizeError}>
        <FormInputNumber
          placeholder={l('minChanSizePlaceholder')}
          extra={Units[Unit.sats].suffix}
          value={orderFormStore.minChanSize}
          onChange={orderFormStore.setMinChanSize}
        />
      </FormField>
      <FormField label={l('feeLabel')} error={orderFormStore.feeRateError}>
        <FormInputNumber
          placeholder={l('feePlaceholder')}
          extra="sats/vByte"
          value={orderFormStore.maxBatchFeeRate}
          onChange={orderFormStore.setMaxBatchFeeRate}
        />
      </FormField>
      <SummaryItem>
        <span>{l('durationLabel')}</span>
        <span className="text-right">
          2016
          <br />
          <Small>(~{l('durationWeeks')})</Small>
        </span>
      </SummaryItem>
      <SummaryItem>
        <span>{l('fixedRateLabel')}</span>
        <span>
          {orderFormStore.perBlockFixedRate < 1
            ? `< 1`
            : `${orderFormStore.perBlockFixedRate}`}
        </span>
      </SummaryItem>
      <ApySummaryItem strong>
        <span>{l('apyLabel')}</span>
        <span>{orderFormStore.apy}%</span>
      </ApySummaryItem>
      <Actions>
        <Button disabled={!orderFormStore.isValid} onClick={orderFormStore.placeOrder}>
          {orderFormStore.placeOrderLabel}
        </Button>
      </Actions>
    </Section>
  );
};

export default observer(OrderFormSection);