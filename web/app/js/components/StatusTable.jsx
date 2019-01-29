import BaseTable from './BaseTable.jsx';
import PropTypes from 'prop-types';
import React from 'react';
import Tooltip from '@material-ui/core/Tooltip';
import _get from 'lodash/get';
import _merge from 'lodash/merge';
import classNames from 'classnames';
import { statusClassNames } from './util/theme.js';
import { withStyles } from '@material-ui/core/styles';

const styles = theme => _merge({}, statusClassNames(theme), {
  statusTableDot: {
    width: 2 * theme.spacing.unit,
    height: 2 * theme.spacing.unit,
    minWidth: 2 * theme.spacing.unit,
    borderRadius: "50%",
    display: "inline-block",
    marginRight: theme.spacing.unit,
  }
});

const columnConfig = {
  "Pod Status": {
    width: 200,
    wrapDotsAt: 7, // dots take up more than one line in the table; space them out
    dotExplanation: status => {
      return status.value === "good" ? "is up and running" : "has not been started";
    }
  },
  "Proxy Status": {
    width: 250,
    wrapDotsAt: 9,
    dotExplanation: pod => {
      let addedStatus = !pod.added ? "Not in mesh" : "Added to mesh";

      return (
        <React.Fragment>
          <div>Pod status: {pod.status}</div>
          <div>{addedStatus}</div>
        </React.Fragment>
      );
    }
  }
};

const StatusDot = ({status, columnName, classes}) => (
  <Tooltip
    placement="top"
    title={(
      <div>
        <div>{status.name}</div>
        <div>{_get(columnConfig, [columnName, "dotExplanation"])(status)}</div>
        <div>Uptime: {status.uptime} ({status.uptimeSec}s)</div>
      </div>
    )}>
    <div
      className={classNames(
        classes.statusTableDot,
        classes[status.value],
      )}
      key={status.name}>&nbsp;
    </div>
  </Tooltip>
);

StatusDot.propTypes = {
  classes: PropTypes.shape({}).isRequired,
  columnName: PropTypes.string.isRequired,
  status: PropTypes.shape({
    name: PropTypes.string.isRequired,
    value: PropTypes.string.isRequired,
  }).isRequired,
};

const columns = {
  resourceName: {
    title: "Deployment",
    dataIndex: "name"
  },
  pods: {
    title: "Pods",
    key: "numEntities",
    isNumeric: true,
    render: d => d.pods.length
  },
  status: (name, classes) => {
    return {
      title: name,
      key: "status",
      render: d => {
        return d.pods.map(status => (
          <StatusDot
            status={status}
            columnName={name}
            classes={classes}
            key={`${status.name}-pod-status`} />
        ));
      }
    };
  }
};

class StatusTable extends React.Component {
  static propTypes = {
    classes: PropTypes.shape({}).isRequired,
    data: PropTypes.arrayOf(PropTypes.shape({
      name: PropTypes.string.isRequired,
      pods: PropTypes.arrayOf(PropTypes.object).isRequired, // TODO: What's the real shape here.
      added: PropTypes.bool,
    })).isRequired,
    statusColumnTitle: PropTypes.string.isRequired,
  }

  render() {
    const { classes, statusColumnTitle, data } = this.props;
    let tableCols = [
      columns.resourceName,
      columns.pods,
      columns.status(statusColumnTitle, classes)
    ];

    return (
      <BaseTable
        tableRows={data}
        tableColumns={tableCols}
        tableClassName="metric-table"
        defaultOrderBy="name"
        rowKey={r => r.name} />
    );
  }
}

export default withStyles(styles, { withTheme: true })(StatusTable);
